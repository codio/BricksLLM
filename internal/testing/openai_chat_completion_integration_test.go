package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/key"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/bricks-cloud/bricksllm/internal/provider/anthropic"
	"github.com/caarlos0/env"
	goopenai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type config struct {
	OpenAiKey    string `env:"OPENAI_API_KEY" envDefault:""`
	AnthropicKey string `env:"ANTHROPIC_API_KEY" envDefault:""`
}

func parseEnvVariables() (*config, error) {
	cfg := &config{}
	err := env.Parse(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func chatCompletionRequest(request *goopenai.ChatCompletionRequest, apiKey string, customId string) (int, []byte, error) {
	jsonData, err := json.Marshal(request)

	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8002/api/providers/openai/v1/chat/completions", io.NopCloser(bytes.NewBuffer(jsonData)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Accept-Encoding", "*/*")

	if len(customId) != 0 {
		req.Header.Set("X-CUSTOM-EVENT-ID", customId)
	}

	client := http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return 0, nil, err
	}

	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, bs, err
}

func completionRequest(request *anthropic.CompletionRequest, apiKey string, customId string) (int, []byte, error) {
	jsonData, err := json.Marshal(request)

	if err != nil {
		return 0, nil, err
	}

	header := map[string][]string{
		"content-type":      {"application/json"},
		"accept":            {"application/json"},
		"x-api-key":         {apiKey},
		"anthropic-version": {"2023-06-01"},
	}

	if len(customId) != 0 {
		header["X-CUSTOM-EVENT-ID"] = []string{customId}
	}

	client := http.Client{}

	resp, err := client.Do(&http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Scheme: "http", Host: "localhost:8002", Path: "/api/providers/anthropic/v1/complete"},
		Header: header,
		Body:   io.NopCloser(bytes.NewBuffer(jsonData)),
	})

	if err != nil {
		return 0, nil, err
	}

	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, bs, err
}

func TestOpenAi_ChatCompletion(t *testing.T) {
	c, _ := parseEnvVariables()
	db := connectToPostgreSqlDb()

	t.Run("when api key is valid", func(t *testing.T) {
		defer deleteEventsTable(db)

		setting := &provider.Setting{
			Provider: "openai",
			Setting: map[string]string{
				"apikey": c.OpenAiKey,
			},
			Name: "test",
		}

		created, err := createProviderSetting(setting)
		require.Nil(t, err)
		defer deleteProviderSetting(db, created.Id)

		requestKey := &key.RequestKey{
			Name:      "Spike's Testing Key",
			Tags:      []string{"spike"},
			Key:       "actualKey",
			SettingId: created.Id,
		}

		createdKey, err := createApiKey(requestKey)
		require.Nil(t, err)
		defer deleteApiKey(db, createdKey.KeyId)

		time.Sleep(6 * time.Second)

		request := &goopenai.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []goopenai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: "hi",
				},
			},
		}

		code, bs, err := chatCompletionRequest(request, requestKey.Key, "")
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, code, string(bs))
	})
}
