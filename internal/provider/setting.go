package provider

import (
	"encoding/json"
	"fmt"
)

const (
	BedrockRegionParam      = "awsRegion"
	BedrockRoleArnParam     = "awsRoleArn"
	BedrockExternalIdParam  = "awsExternalId"
	BedrockSessionTagsParam = "awsSessionTags"
	BedrockModelsParam      = "bedrockModels"
)

func ProviderSupportsBedrockIam(provider string) bool {
	return provider == "openai" || provider == "anthropic"
}

func UsesBedrockIam(provider string, params map[string]string) bool {
	return ProviderSupportsBedrockIam(provider) && len(params[BedrockRoleArnParam]) != 0
}

func ParseBedrockStringMap(params map[string]string, name string) (map[string]string, error) {
	raw := params[name]
	if len(raw) == 0 {
		return nil, nil
	}

	parsed := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of strings: %w", name, err)
	}

	return parsed, nil
}

type Setting struct {
	CreatedAt     int64             `json:"createdAt"`
	UpdatedAt     int64             `json:"updatedAt"`
	Provider      string            `json:"provider"`
	Setting       map[string]string `json:"setting,omitempty"`
	Id            string            `json:"id"`
	Name          string            `json:"name"`
	AllowedModels []string          `json:"allowedModels"`
	CostMap       *CostMap          `json:"costMap"`
}

type CostMap struct {
	PromptCostPerModel     map[string]float64 `json:"promptCostPerModel"`
	CompletionCostPerModel map[string]float64 `json:"completionCostPerModel"`
	EmbeddingsCostPerModel map[string]float64 `json:"embeddingsCostPerModel"`
}

func (s *Setting) GetParam(key string) string {
	return s.Setting[key]
}

func (s *Setting) UsesBedrockIam() bool {
	return UsesBedrockIam(s.Provider, s.Setting)
}

type UpdateSetting struct {
	UpdatedAt     int64             `json:"updatedAt"`
	Setting       map[string]string `json:"setting,omitempty"`
	Name          *string           `json:"name"`
	AllowedModels *[]string         `json:"allowedModels,omitempty"`
	CostMap       *CostMap          `json:"costMap,omitempty"`
}

func EstimateCostWithCostMap(model string, tks int, div float64, costMap map[string]float64) (float64, error) {
	cost, ok := costMap[model]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	tksInFloat := float64(tks)
	return tksInFloat / div * cost, nil
}

func EstimateTotalCostWithCostMaps(model string, ptks, ctks int, div float64, promptCostMap map[string]float64, completionCostMap map[string]float64) (float64, error) {
	pcost, ok := promptCostMap[model]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	ccost, ok := completionCostMap[model]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	ptksInFloat := float64(ptks)
	ctksInFloat := float64(ctks)

	return ptksInFloat/div*pcost + ctksInFloat/div*ccost, nil
}
