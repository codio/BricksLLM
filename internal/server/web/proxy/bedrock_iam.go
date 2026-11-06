package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bricks-cloud/bricksllm/internal/bedrockiam"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/gin-gonic/gin"
)

const bedrockTargetContextKey = "bedrockIamTarget"

type requestSigner interface {
	Sign(ctx context.Context, req *http.Request, body []byte, target bedrockiam.Target) error
}

type bedrockIamTarget struct {
	iam    bedrockiam.Target
	models map[string]string
}

var bedrockForwardedHeaders = []string{"Content-Type", "anthropic-version", "anthropic-beta"}

func bedrockIamTargetFromSetting(s *provider.Setting) (*bedrockIamTarget, error) {
	if s == nil || !s.UsesBedrockIam() {
		return nil, nil
	}

	tags, err := provider.ParseBedrockStringMap(s.Setting, provider.BedrockSessionTagsParam)
	if err != nil {
		return nil, err
	}

	models, err := provider.ParseBedrockStringMap(s.Setting, provider.BedrockModelsParam)
	if err != nil {
		return nil, err
	}

	return &bedrockIamTarget{
		iam: bedrockiam.Target{
			Region:      s.Setting[provider.BedrockRegionParam],
			RoleArn:     s.Setting[provider.BedrockRoleArnParam],
			ExternalId:  s.Setting[provider.BedrockExternalIdParam],
			SessionName: "bricksllm-" + s.Id,
			SessionTags: tags,
		},
		models: models,
	}, nil
}

func bedrockRuntimeUrl(region, path string) string {
	return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com%s", region, path)
}

func mapRequestModel(body []byte, models map[string]string) ([]byte, error) {
	if len(models) == 0 {
		return body, nil
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}

	rawModel, ok := fields["model"]
	if !ok {
		return body, nil
	}

	var model string
	if err := json.Unmarshal(rawModel, &model); err != nil {
		return nil, err
	}

	mapped, ok := models[model]
	if !ok {
		return body, nil
	}

	encodedModel, err := json.Marshal(mapped)
	if err != nil {
		return nil, err
	}
	fields["model"] = encodedModel

	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(fields); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

func newProviderRequest(ctx context.Context, c *gin.Context, signer requestSigner, directUrl, bedrockPath string) (*http.Request, error) {
	raw, exists := c.Get(bedrockTargetContextKey)
	if !exists {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, directUrl, c.Request.Body)
		if err != nil {
			return nil, err
		}

		copyHttpHeaders(c.Request, req, c.GetBool("removeUserAgent"))

		if c.GetBool("stream") {
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Cache-Control", "no-cache")
			req.Header.Set("Connection", "keep-alive")
		}

		return req, nil
	}

	target, ok := raw.(*bedrockIamTarget)
	if !ok || target == nil {
		return nil, errors.New("bedrock target in context has unexpected type")
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}

	body, err = mapRequestModel(body, target.models)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bedrockRuntimeUrl(target.iam.Region, bedrockPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for _, h := range bedrockForwardedHeaders {
		if v := c.Request.Header.Get(h); len(v) != 0 {
			req.Header.Set(h, v)
		}
	}

	if len(req.Header.Get("Content-Type")) == 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.GetBool("stream") {
		req.Header.Set("Accept", "text/event-stream")
	}

	if err := signer.Sign(ctx, req, body, target.iam); err != nil {
		return nil, err
	}

	return req, nil
}
