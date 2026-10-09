package anthropic

import (
	"fmt"
	"strings"

	"github.com/bricks-cloud/bricksllm/internal/provider"
)

var AnthropicPerMillionTokenCost = map[string]map[string]float64{
	"prompt": {
		"claude-mythos-5-1": 10.0,
		"claude-mythos-5":   10.0,

		"claude-fable-5-1": 10.0,
		"claude-fable-5":   10.0,

		"claude-opus-5-5": 4.0,
		"claude-opus-5":   5.0,
		"claude-opus-4-8": 5.0,
		"claude-opus-4-7": 5.0,
		"claude-opus-4-6": 5.0,
		"claude-opus-4-5": 5.0,
		"claude-opus-4-1": 15.0,
		"claude-opus-4":   15.0,

		"claude-sonnet-5-5": 2.0,
		"claude-sonnet-5":   2.0,
		"claude-sonnet-4-6": 3.0,
		"claude-sonnet-4-5": 3.0,
		"claude-sonnet-4":   3.0,

		"claude-haiku-5-5":       0.5,
		"claude-haiku-5-5~long":  0.5,
		"claude-haiku-5-5~short": 0.1,
		"claude-haiku-4-5":       1.0,

		// old models
		"claude-3-7-sonnet": 3.0,
		"claude-3-5-haiku":  0.8,
		"claude-3-haiku":    0.25,
		"claude-3-5-sonnet": 3.0,
		"claude-3-opus":     15.0,
	},
	"completion": {
		"claude-mythos-5-1": 50.0,
		"claude-mythos-5":   50.0,

		"claude-fable-5-1": 50.0,
		"claude-fable-5":   50.0,

		"claude-opus-5-5": 20.0,
		"claude-opus-5":   25.0,
		"claude-opus-4-8": 25.0,
		"claude-opus-4-7": 25.0,
		"claude-opus-4-6": 25.0,
		"claude-opus-4-5": 25.0,
		"claude-opus-4-1": 75.0,
		"claude-opus-4":   75.0,

		"claude-sonnet-5-5": 10.0,
		"claude-sonnet-5":   10.0,
		"claude-sonnet-4-6": 15.0,
		"claude-sonnet-4-5": 15.0,
		"claude-sonnet-4":   15.0,

		"claude-haiku-5-5":       2.5,
		"claude-haiku-5-5~long":  2.5,
		"claude-haiku-5-5~short": 0.5,
		"claude-haiku-4-5":       5.0,

		// old models
		"claude-3-7-sonnet": 15.0,
		"claude-3-5-haiku":  4.0,
		"claude-3-haiku":    1.25,
		"claude-3-5-sonnet": 15.0,
		"claude-3-opus":     75.0,
	},
}

type tokenCounter interface {
	Count(input string) int
}

type CostEstimator struct {
	tokenCostMap map[string]map[string]float64
	tc           tokenCounter
}

func NewCostEstimator(tc tokenCounter) *CostEstimator {
	return &CostEstimator{
		tokenCostMap: AnthropicPerMillionTokenCost,
		tc:           tc,
	}
}

func (ce *CostEstimator) EstimateTotalCost(model string, promptTks, completionTks int) (float64, error) {
	normalized := normalizeAnthropicModel(model)
	modelWithCtx := provider.ModelWithContextLength(normalized, int64(promptTks))

	promptCost, err := ce.estimateCost("prompt", modelWithCtx, promptTks)
	if err != nil {
		return 0, err
	}

	completionCost, err := ce.estimateCost("completion", modelWithCtx, completionTks)
	if err != nil {
		return 0, err
	}

	return promptCost + completionCost, nil
}

// normalizeAnthropicModel strips version/date suffixes (and Bedrock's
// cross-region "us.anthropic...." prefix) down to the base model family name
// used as a key in AnthropicPerMillionTokenCost.
func normalizeAnthropicModel(model string) string {
	if strings.HasPrefix(model, "us") {
		return convertAmazonModelToAnthropicModel(model)
	}
	return SelectModel(model)
}

func (ce *CostEstimator) estimateCost(costMapKey, model string, tks int) (float64, error) {
	costMap, ok := ce.tokenCostMap[costMapKey]
	if !ok {
		return 0, fmt.Errorf("%s token cost is not provided", costMapKey)
	}

	cost, ok := costMap[model]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	tksInFloat := float64(tks)
	return tksInFloat / 1000000 * cost, nil
}

func (ce *CostEstimator) EstimatePromptCost(model string, tks int) (float64, error) {
	modelWithCtx := provider.ModelWithContextLength(normalizeAnthropicModel(model), int64(tks))
	return ce.estimateCost("prompt", modelWithCtx, tks)
}

func SelectModel(model string) string {
	if strings.HasPrefix(model, "claude-mythos-5-1") {
		return "claude-mythos-5-1"
	}
	if strings.HasPrefix(model, "claude-mythos-5") {
		return "claude-mythos-5"
	}
	if strings.HasPrefix(model, "claude-fable-5-1") {
		return "claude-fable-5-1"
	}
	if strings.HasPrefix(model, "claude-fable-5") {
		return "claude-fable-5"
	}
	if strings.HasPrefix(model, "claude-opus-5-5") {
		return "claude-opus-5-5"
	}
	if strings.HasPrefix(model, "claude-opus-5") {
		return "claude-opus-5"
	}
	if strings.HasPrefix(model, "claude-opus-4-8") {
		return "claude-opus-4-8"
	}
	if strings.HasPrefix(model, "claude-opus-4-7") {
		return "claude-opus-4-7"
	}
	if strings.HasPrefix(model, "claude-opus-4-6") {
		return "claude-opus-4-6"
	}
	if strings.HasPrefix(model, "claude-opus-4-5") {
		return "claude-opus-4-5"
	}
	if strings.HasPrefix(model, "claude-opus-4-1") {
		return "claude-opus-4-1"
	}
	if strings.HasPrefix(model, "claude-opus-4") {
		return "claude-opus-4"
	}
	if strings.HasPrefix(model, "claude-sonnet-5-5") {
		return "claude-sonnet-5-5"
	}
	if strings.HasPrefix(model, "claude-sonnet-5") {
		return "claude-sonnet-5"
	}
	if strings.HasPrefix(model, "claude-sonnet-4-6") {
		return "claude-sonnet-4-6"
	}
	if strings.HasPrefix(model, "claude-sonnet-4-5") {
		return "claude-sonnet-4-5"
	}
	if strings.HasPrefix(model, "claude-sonnet-4") {
		return "claude-sonnet-4"
	}
	if strings.HasPrefix(model, "claude-haiku-5-5") {
		return "claude-haiku-5-5"
	}
	if strings.HasPrefix(model, "claude-haiku-4-5") {
		return "claude-haiku-4-5"
	}

	// old
	if strings.HasPrefix(model, "claude-3-7-sonnet") {
		return "claude-3-7-sonnet"
	}
	if strings.HasPrefix(model, "claude-3-5-haiku") {
		return "claude-3-5-haiku"
	}
	if strings.HasPrefix(model, "claude-3-5-sonnet") {
		return "claude-3-5-sonnet"
	}
	if strings.HasPrefix(model, "claude-3-opus") {
		return "claude-3-opus"
	}
	if strings.HasPrefix(model, "claude-3-haiku") {
		return "claude-3-haiku"
	}
	return model
}

func convertAmazonModelToAnthropicModel(model string) string {
	parts := strings.Split(model, ".")
	if len(parts) < 3 {
		return model
	}

	return SelectModel(parts[2])
}

func (ce *CostEstimator) EstimateCompletionCost(model string, promptTks, completionTks int) (float64, error) {
	modelWithCtx := provider.ModelWithContextLength(normalizeAnthropicModel(model), int64(promptTks))
	return ce.estimateCost("completion", modelWithCtx, completionTks)
}

func (ce *CostEstimator) Count(input string) int {
	return ce.tc.Count(input)
}

var (
	anthropicMessageOverhead = 4
)

func (ce *CostEstimator) CountMessagesTokens(messages []Message) int {
	count := 0

	for _, message := range messages {
		count += ce.tc.Count(message.Content.String()) + anthropicMessageOverhead
	}

	return count + anthropicMessageOverhead
}
