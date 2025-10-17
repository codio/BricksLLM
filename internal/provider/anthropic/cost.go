package anthropic

import (
	"errors"
	"fmt"
	"strings"
)

var AnthropicPerMillionTokenCost = map[string]map[string]float64{
	"prompt": {
		"claude-sonnet-4.5": 3.0,
		"claude-sonnet-4":   3.0,
		"claude-3.7-sonnet": 3.0,
		"claude-opus-4.1":   15.0,
		"claude-opus-4":     15.0,
		"claude-3.5-haiku":  0.8,
		"claude-3-haiku":    0.25,

		"claude-3.5-sonnet": 3.0,
		"claude-3-opus":     15.0,
	},
	"completion": {
		"claude-sonnet-4.5": 15.0,
		"claude-sonnet-4":   15.0,
		"claude-3.7-sonnet": 15.0,
		"claude-opus-4.1":   75.0,
		"claude-opus-4":     75.0,
		"claude-3.5-haiku":  4.0,
		"claude-3-haiku":    1.25,

		"claude-3.5-sonnet": 15.0,
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
	promptCost, err := ce.EstimatePromptCost(model, promptTks)
	if err != nil {
		return 0, err
	}

	completionCost, err := ce.EstimateCompletionCost(model, completionTks)
	if err != nil {
		return 0, err
	}

	return promptCost + completionCost, nil
}

func (ce *CostEstimator) EstimatePromptCost(model string, tks int) (float64, error) {
	costMap, ok := ce.tokenCostMap["prompt"]
	if !ok {
		return 0, errors.New("prompt token cost is not provided")

	}

	selected := ""
	if strings.HasPrefix(model, "us") {
		selected = convertAmazonModelToAnthropicModel(model)
	} else {
		selected = SelectModel(model)
	}

	cost, ok := costMap[selected]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	tksInFloat := float64(tks)
	return tksInFloat / 1000000 * cost, nil
}

func SelectModel(model string) string {
	if strings.HasPrefix(model, "claude-sonnet-4.5") || strings.HasPrefix(model, "claude-sonnet-4-5") {
		return "claude-sonnet-4.5"
	} else if strings.HasPrefix(model, "claude-sonnet-4") {
		return "claude-sonnet-4"
	} else if strings.HasPrefix(model, "claude-3.7-sonnet") || strings.HasPrefix(model, "claude-3-7-sonnet") {
		return "claude-3.7-sonnet"
	} else if strings.HasPrefix(model, "claude-opus-4.1") || strings.HasPrefix(model, "claude-opus-4-1") {
		return "claude-opus-4.1"
	} else if strings.HasPrefix(model, "claude-opus-4") {
		return "claude-opus-4"
	} else if strings.HasPrefix(model, "claude-3.5-haiku") || strings.HasPrefix(model, "claude-3-5-haiku") {
		return "claude-3.5-haiku"
	} else if strings.HasPrefix(model, "claude-3-haiku") {
		return "claude-3-haiku"
	} else if strings.HasPrefix(model, "claude-3.5-sonnet") || strings.HasPrefix(model, "claude-3-5-sonnet") {
		return "claude-3.5-sonnet"
	} else if strings.HasPrefix(model, "claude-3-opus") {
		return "claude-3-opus"
	}
	return ""
}

func convertAmazonModelToAnthropicModel(model string) string {
	parts := strings.Split(model, ".")
	if len(parts) < 3 {
		return model
	}

	return SelectModel(parts[2])
}

func (ce *CostEstimator) EstimateCompletionCost(model string, tks int) (float64, error) {
	costMap, ok := ce.tokenCostMap["completion"]
	if !ok {
		return 0, errors.New("prompt token cost is not provided")
	}

	selected := ""
	if strings.HasPrefix(model, "us") {
		selected = convertAmazonModelToAnthropicModel(model)
	} else {
		selected = SelectModel(model)
	}

	cost, ok := costMap[selected]
	if !ok {
		return 0, errors.New("model is not present in the cost map provided")
	}

	tksInFloat := float64(tks)
	return tksInFloat / 1000000 * cost, nil
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
		count += ce.tc.Count(message.Content) + anthropicMessageOverhead
	}

	return count + anthropicMessageOverhead
}
