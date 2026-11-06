package manager

import (
	"testing"

	"github.com/bricks-cloud/bricksllm/internal/provider"
)

func TestValidateSettingsBedrockIam(t *testing.T) {
	m := &ProviderSettingsManager{}
	role := "arn:aws:iam::123456789012:role/bricks"

	tests := []struct {
		name     string
		provider string
		params   map[string]string
		wantErr  bool
	}{
		{"anthropic role and region without apikey", "anthropic", map[string]string{provider.BedrockRoleArnParam: role, provider.BedrockRegionParam: "us-east-1"}, false},
		{"openai role and region without apikey", "openai", map[string]string{provider.BedrockRoleArnParam: role, provider.BedrockRegionParam: "us-east-1"}, false},
		{"role without region", "anthropic", map[string]string{provider.BedrockRoleArnParam: role}, true},
		{"invalid session tags", "anthropic", map[string]string{provider.BedrockRoleArnParam: role, provider.BedrockRegionParam: "us-east-1", provider.BedrockSessionTagsParam: "service=opencode"}, true},
		{"invalid models map", "openai", map[string]string{provider.BedrockRoleArnParam: role, provider.BedrockRegionParam: "us-east-1", provider.BedrockModelsParam: `{"a":1}`}, true},
		{"valid json maps", "openai", map[string]string{provider.BedrockRoleArnParam: role, provider.BedrockRegionParam: "us-east-1", provider.BedrockSessionTagsParam: `{"service":"opencode"}`, provider.BedrockModelsParam: `{"gpt-5.5":"openai.gpt-5.5"}`}, false},
		{"direct anthropic still requires apikey", "anthropic", map[string]string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.validateSettings(tt.provider, tt.params)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
