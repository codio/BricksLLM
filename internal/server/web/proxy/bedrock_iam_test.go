package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bricks-cloud/bricksllm/internal/bedrockiam"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/gin-gonic/gin"
)

type fakeSigner struct {
	body   []byte
	target bedrockiam.Target
	err    error
	calls  int
}

func (f *fakeSigner) Sign(_ context.Context, req *http.Request, body []byte, target bedrockiam.Target) error {
	f.calls++
	f.body = body
	f.target = target
	if f.err != nil {
		return f.err
	}

	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 signed")
	return nil
}

func newTestContext(body string, headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/providers/anthropic/v1/messages", strings.NewReader(body))
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}

	return c
}

func bedrockSetting(params map[string]string) *provider.Setting {
	setting := map[string]string{
		provider.BedrockRegionParam:  "us-east-1",
		provider.BedrockRoleArnParam: "arn:aws:iam::123456789012:role/bricks-opencode",
	}
	for k, v := range params {
		setting[k] = v
	}

	return &provider.Setting{Id: "setting-1", Provider: "anthropic", Setting: setting}
}

func TestMapRequestModel(t *testing.T) {
	models := map[string]string{"claude-sonnet-4-5": "us.anthropic.claude-sonnet-4-5-20250929-v1:0"}

	tests := []struct {
		name string
		body string
		want string
	}{
		{"mapped", `{"model":"claude-sonnet-4-5","max_tokens":10}`, "us.anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{"unknown model passes through", `{"model":"openai.gpt-5.5"}`, "openai.gpt-5.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := mapRequestModel([]byte(tt.body), models)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var parsed map[string]any
			if err := json.Unmarshal(out, &parsed); err != nil {
				t.Fatalf("invalid json: %v", err)
			}

			if parsed["model"] != tt.want {
				t.Fatalf("got model %v, want %s", parsed["model"], tt.want)
			}
		})
	}
}

func TestMapRequestModelKeepsOtherFieldsAndHtml(t *testing.T) {
	body := `{"model":"claude-sonnet-4-5","system":"use <tags> & stuff","messages":[{"role":"user","content":"hi"}],"stream":true}`

	out, err := mapRequestModel([]byte(body), map[string]string{"claude-sonnet-4-5": "us.anthropic.claude-sonnet-4-5-v1:0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(out), `"use <tags> & stuff"`) {
		t.Fatalf("expected html characters to be preserved, got %s", out)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if parsed["stream"] != true || len(parsed["messages"].([]any)) != 1 {
		t.Fatalf("expected other fields to be preserved, got %s", out)
	}
}

func TestMapRequestModelWithoutMapReturnsBodyUnchanged(t *testing.T) {
	body := []byte(`{"model":"x"}`)

	out, err := mapRequestModel(body, nil)
	if err != nil || string(out) != string(body) {
		t.Fatalf("expected unchanged body, got %s, %v", out, err)
	}
}

func TestBedrockIamTargetFromSetting(t *testing.T) {
	s := bedrockSetting(map[string]string{
		provider.BedrockExternalIdParam:  "ext",
		provider.BedrockSessionTagsParam: `{"service":"opencode"}`,
		provider.BedrockModelsParam:      `{"claude-sonnet-4-5":"us.anthropic.claude-sonnet-4-5-v1:0"}`,
	})

	target, err := bedrockIamTargetFromSetting(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target.iam.Region != "us-east-1" || target.iam.RoleArn != "arn:aws:iam::123456789012:role/bricks-opencode" || target.iam.ExternalId != "ext" {
		t.Fatalf("unexpected target %+v", target.iam)
	}

	if target.iam.SessionName != "bricksllm-setting-1" || target.iam.SessionTags["service"] != "opencode" {
		t.Fatalf("unexpected session %+v", target.iam)
	}

	if target.models["claude-sonnet-4-5"] != "us.anthropic.claude-sonnet-4-5-v1:0" {
		t.Fatalf("unexpected models %+v", target.models)
	}
}

func TestBedrockIamTargetFromSettingWithoutRole(t *testing.T) {
	target, err := bedrockIamTargetFromSetting(&provider.Setting{Provider: "anthropic", Setting: map[string]string{"apikey": "sk"}})
	if err != nil || target != nil {
		t.Fatalf("expected no target, got %+v, %v", target, err)
	}
}

func TestBedrockIamTargetFromSettingInvalidJson(t *testing.T) {
	_, err := bedrockIamTargetFromSetting(bedrockSetting(map[string]string{provider.BedrockModelsParam: `["not","a","map"]`}))
	if err == nil {
		t.Fatal("expected error for invalid models map")
	}
}

func TestNewProviderRequestDirect(t *testing.T) {
	c := newTestContext(`{"model":"claude-sonnet-4-5"}`, map[string]string{"x-api-key": "sk-ant", "anthropic-version": "2023-06-01"})
	c.Set("stream", true)
	signer := &fakeSigner{}

	req, err := newProviderRequest(context.Background(), c, signer, "https://api.anthropic.com/v1/messages", "/anthropic/v1/messages")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.URL.String() != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("unexpected url %s", req.URL)
	}

	if req.Header.Get("x-api-key") != "sk-ant" || req.Header.Get("Accept") != "text/event-stream" {
		t.Fatalf("expected copied headers and streaming headers, got %v", req.Header)
	}

	if signer.calls != 0 {
		t.Fatal("direct request must not be signed")
	}
}

func TestNewProviderRequestBedrock(t *testing.T) {
	c := newTestContext(`{"model":"claude-sonnet-4-5","max_tokens":10}`, map[string]string{
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    "interleaved-thinking-2025-05-14",
		"User-Agent":        "opencode",
		"X-Stainless-Os":    "Linux",
	})
	c.Set("stream", true)

	target, err := bedrockIamTargetFromSetting(bedrockSetting(map[string]string{
		provider.BedrockModelsParam: `{"claude-sonnet-4-5":"us.anthropic.claude-sonnet-4-5-v1:0"}`,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.Set(bedrockTargetContextKey, target)

	signer := &fakeSigner{}
	req, err := newProviderRequest(context.Background(), c, signer, "https://api.anthropic.com/v1/messages", "/anthropic/v1/messages")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.URL.String() != "https://bedrock-runtime.us-east-1.amazonaws.com/anthropic/v1/messages" {
		t.Fatalf("unexpected url %s", req.URL)
	}

	for h, want := range map[string]string{
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    "interleaved-thinking-2025-05-14",
		"Content-Type":      "application/json",
		"Accept":            "text/event-stream",
		"Authorization":     "AWS4-HMAC-SHA256 signed",
	} {
		if got := req.Header.Get(h); got != want {
			t.Fatalf("header %s: got %q, want %q", h, got, want)
		}
	}

	for _, h := range []string{"User-Agent", "X-Stainless-Os", "Connection", "Accept-Encoding"} {
		if v := req.Header.Get(h); v != "" {
			t.Fatalf("header %s must not be forwarded to bedrock, got %q", h, v)
		}
	}

	sent, _ := io.ReadAll(req.Body)
	if string(sent) != string(signer.body) {
		t.Fatalf("signed body differs from sent body:\nsigned %s\nsent   %s", signer.body, sent)
	}

	if !strings.Contains(string(sent), `"us.anthropic.claude-sonnet-4-5-v1:0"`) {
		t.Fatalf("expected mapped model in body, got %s", sent)
	}

	if signer.target.RoleArn != "arn:aws:iam::123456789012:role/bricks-opencode" {
		t.Fatalf("unexpected signing target %+v", signer.target)
	}
}

func TestNewProviderRequestBedrockSignError(t *testing.T) {
	c := newTestContext(`{"model":"m"}`, nil)
	target, _ := bedrockIamTargetFromSetting(bedrockSetting(nil))
	c.Set(bedrockTargetContextKey, target)

	_, err := newProviderRequest(context.Background(), c, &fakeSigner{err: errors.New("access denied")}, "https://api.openai.com/v1/responses", "/openai/v1/responses")
	if err == nil {
		t.Fatal("expected sign error to be returned")
	}
}
