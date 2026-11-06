package auth

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bricks-cloud/bricksllm/internal/key"
	"github.com/bricks-cloud/bricksllm/internal/provider"
)

type stubKeysCache struct {
	key *key.ResponseKey
}

func (s stubKeysCache) GetKeyViaCache(string) (*key.ResponseKey, error) {
	return s.key, nil
}

func (s stubKeysCache) GetKeyHashBySecondary(string) (string, error) {
	return "", nil
}

type stubSettingsManager struct {
	settings map[string]*provider.Setting
}

func (s stubSettingsManager) GetSettingViaCache(id string) (*provider.Setting, error) {
	return s.settings[id], nil
}

func (s stubSettingsManager) GetSettingsViaCache([]string) ([]*provider.Setting, error) {
	return nil, nil
}

func TestAuthenticateHttpRequestDoesNotEchoRawKeyWhenNoSettingMatches(t *testing.T) {
	raw := "0123456789abcdef0123456789abcdef"
	a := NewAuthenticator(
		stubSettingsManager{settings: map[string]*provider.Setting{"s1": bedrockIamSetting("openai")}},
		stubKeysCache{key: &key.ResponseKey{KeyId: "k1", SettingIds: []string{"s1"}}},
		nil, nil, nil,
	)

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/api/providers/openai/v1/embeddings", nil)
	req.Header.Set("Authorization", "Bearer "+raw)

	_, _, err := a.AuthenticateHttpRequest(req, "")
	if err == nil {
		t.Fatal("expected an auth error")
	}

	if strings.Contains(err.Error(), raw) || strings.Contains(err.Error(), raw[5:]) {
		t.Fatalf("error message leaks the raw key: %s", err.Error())
	}

	if !strings.Contains(err.Error(), "provider setting not found for key 01234") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

type disabledDecryptor struct{}

func (disabledDecryptor) Decrypt(input string, _ map[string]string) (string, error) {
	return input, nil
}

func (disabledDecryptor) Enabled() bool {
	return false
}

func TestAuthenticateHttpRequestRotationPutsUsedSettingFirst(t *testing.T) {
	direct := &provider.Setting{Id: "direct", Provider: "anthropic", Setting: map[string]string{"apikey": "sk-ant"}}
	iam := bedrockIamSetting("anthropic")
	iam.Id = "iam"

	a := NewAuthenticator(
		stubSettingsManager{settings: map[string]*provider.Setting{"direct": direct, "iam": iam}},
		stubKeysCache{key: &key.ResponseKey{KeyId: "k1", SettingIds: []string{"direct", "iam"}, RotationEnabled: true}},
		nil, nil, disabledDecryptor{},
	)

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		req, _ := http.NewRequest(http.MethodPost, "http://localhost/api/providers/anthropic/v1/messages", nil)
		req.Header.Set("x-api-key", "bricks-key")

		_, selected, err := a.AuthenticateHttpRequest(req, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(selected) != 2 {
			t.Fatalf("expected both settings to be selected, got %d", len(selected))
		}

		first := selected[0]
		seen[first.Id] = true

		switch first.Id {
		case "direct":
			if req.Header.Get("x-api-key") != "sk-ant" {
				t.Fatalf("direct setting first but x-api-key is %q", req.Header.Get("x-api-key"))
			}
		case "iam":
			if req.Header.Get("x-api-key") != "" {
				t.Fatalf("bedrock iam setting first but x-api-key is %q", req.Header.Get("x-api-key"))
			}
		default:
			t.Fatalf("unexpected first setting %q", first.Id)
		}
	}

	if !seen["direct"] || !seen["iam"] {
		t.Fatalf("expected rotation to pick both settings, saw %v", seen)
	}
}

func bedrockIamSetting(providerName string) *provider.Setting {
	return &provider.Setting{
		Provider: providerName,
		Setting: map[string]string{
			provider.BedrockRegionParam:  "us-east-1",
			provider.BedrockRoleArnParam: "arn:aws:iam::123456789012:role/bricks",
		},
	}
}

func TestCanSettingAccessPathBedrockIam(t *testing.T) {
	tests := []struct {
		provider string
		path     string
		want     bool
	}{
		{"anthropic", "/api/providers/anthropic/v1/messages", true},
		{"anthropic", "/api/providers/anthropic/v1/complete", false},
		{"anthropic", "/api/providers/openai/v1/chat/completions", false},
		{"openai", "/api/providers/openai/v1/chat/completions", true},
		{"openai", "/api/providers/openai/v1/responses", true},
		{"openai", "/api/providers/openai/v1/responses/resp_1/cancel", true},
		{"openai", "/api/providers/openai/v1/embeddings", false},
		{"openai", "/api/providers/openai/v1/models", false},
		{"openai", "/api/providers/anthropic/v1/messages", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider+tt.path, func(t *testing.T) {
			if got := canSettingAccessPath(bedrockIamSetting(tt.provider), tt.path); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanSettingAccessPathWithoutBedrockIam(t *testing.T) {
	s := &provider.Setting{Provider: "openai", Setting: map[string]string{"apikey": "sk"}}

	if !canSettingAccessPath(s, "/api/providers/openai/v1/embeddings") {
		t.Fatal("expected direct openai setting to keep access to all openai paths")
	}
}

func TestRewriteHttpAuthHeaderBedrockIamStripsClientCredentials(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://localhost/api/providers/anthropic/v1/messages", nil)
	req.Header.Set("x-api-key", "bricks-key")
	req.Header.Set("Authorization", "Bearer bricks-key")
	req.Header.Set("api-key", "bricks-key")

	if err := rewriteHttpAuthHeader(req, bedrockIamSetting("anthropic")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, h := range []string{"x-api-key", "Authorization", "api-key"} {
		if v := req.Header.Get(h); v != "" {
			t.Fatalf("expected %s to be removed, got %q", h, v)
		}
	}
}

func TestRewriteHttpAuthHeaderDirectAnthropicKeepsApiKey(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://localhost/api/providers/anthropic/v1/messages", nil)
	req.Header.Set("x-api-key", "bricks-key")

	s := &provider.Setting{Provider: "anthropic", Setting: map[string]string{"apikey": "sk-ant"}}
	if err := rewriteHttpAuthHeader(req, s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("x-api-key") != "sk-ant" {
		t.Fatalf("expected provider key, got %q", req.Header.Get("x-api-key"))
	}
}
