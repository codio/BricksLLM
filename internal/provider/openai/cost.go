package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/bricks-cloud/bricksllm/internal/util"
	responsesOpenai "github.com/openai/openai-go/responses"
	goopenai "github.com/sashabaranov/go-openai"
)

func useFinetuneModel(model string) string {
	if isFinetuneModel(model) {
		return parseFinetuneModel(model)
	}

	return model
}

func isFinetuneModel(model string) bool {
	return strings.HasPrefix(model, "ft:")
}

func parseFinetuneModel(model string) string {
	parts := strings.Split(model, ":")
	if len(parts) > 2 {
		return "finetune-" + parts[1]
	}

	return model
}

var OpenAiPerThousandTokenCost = map[string]map[string]float64{
	"prompt": {
		"gpt-5":                        0.001250,
		"gpt-5-mini":                   0.000250,
		"gpt-5-nano":                   0.000050,
		"gpt-5-chat-latest":            0.001250,
		"gpt-5-codex":                  0.001250,
		"gpt-5-pro":                    0.015000,
		"gpt-4.1":                      0.002000,
		"gpt-4.1-mini":                 0.000400,
		"gpt-4.1-nano":                 0.000100,
		"gpt-4o":                       0.002500,
		"gpt-4o-2024-05-13":            0.005000,
		"gpt-4o-mini":                  0.000150,
		"gpt-realtime":                 0.004000,
		"gpt-realtime-mini":            0.000600,
		"gpt-4o-realtime-preview":      0.005000,
		"gpt-4o-mini-realtime-preview": 0.000600,
		"gpt-audio":                    0.002500,
		"gpt-audio-mini":               0.000600,
		"gpt-4o-audio-preview":         0.002500,
		"gpt-4o-mini-audio-preview":    0.000150,
		"o1":                           0.015000,
		"o1-pro":                       0.150000,
		"o3-pro":                       0.020000,
		"o3":                           0.002000,
		"o3-deep-research":             0.010000,
		"o4-mini":                      0.001100,
		"o4-mini-deep-research":        0.002000,
		"o3-mini":                      0.001100,
		"o1-mini":                      0.001100,
		"codex-mini-latest":            0.001500,
		"gpt-4o-mini-search-preview":   0.000150,
		"gpt-4o-search-preview":        0.002500,
		"computer-use-preview":         0.003000,
		"chatgpt-4o-latest":            0.005000,
		"gpt-4-turbo-2024-04-09":       0.010000,
		"gpt-4-0125-preview":           0.010000,
		"gpt-4-1106-preview":           0.010000,
		"gpt-4-1106-vision-preview":    0.010000,
		"gpt-4-0613":                   0.030000,
		"gpt-4-0314":                   0.030000,
		"gpt-4-32k":                    0.060000,
		"gpt-3.5-turbo":                0.000500,
		"gpt-3.5-turbo-0125":           0.000500,
		"gpt-3.5-turbo-1106":           0.001000,
		"gpt-3.5-turbo-0613":           0.001500,
		"gpt-3.5-0301":                 0.001500,
		"gpt-3.5-turbo-instruct":       0.001500,
		"gpt-3.5-turbo-16k-0613":       0.003000,
		"davinci-002":                  0.002000,
		"babbage-002":                  0.000400,
	},
	"cached-prompt": {
		"gpt-5":                        0.000125,
		"gpt-5-mini":                   0.000025,
		"gpt-5-nano":                   0.000005,
		"gpt-5-chat-latest":            0.000125,
		"gpt-5-codex":                  0.000125,
		"gpt-4.1":                      0.000500,
		"gpt-4.1-mini":                 0.000100,
		"gpt-4.1-nano":                 0.000025,
		"gpt-4o":                       0.001250,
		"gpt-4o-mini":                  0.000075,
		"gpt-realtime":                 0.000400,
		"gpt-realtime-mini":            0.000060,
		"gpt-4o-realtime-preview":      0.002500,
		"gpt-4o-mini-realtime-preview": 0.000300,
		"o1":                           0.007500,
		"o3":                           0.000500,
		"o3-deep-research":             0.002500,
		"o4-mini":                      0.000275,
		"o4-mini-deep-research":        0.000500,
		"o3-mini":                      0.000550,
		"o1-mini":                      0.000550,
		"codex-mini-latest":            0.000375,
	},
	"finetune": {
		"gpt-4-0613":         0.09,
		"gpt-3.5-turbo-0125": 0.008,
		"gpt-3.5-turbo-1106": 0.008,
		"gpt-3.5-turbo-0613": 0.008,
		"babbage-002":        0.0004,
		"davinci-002":        0.006,
	},
	"embeddings": {
		"text-embedding-ada-002": 0.0001,
		"text-embedding-3-small": 0.00002,
		"text-embedding-3-large": 0.00013,
	},
	"audio": {
		"whisper-1": 0.006,
		"tts-1":     0.015,
		"tts-1-hd":  0.03,
	},
	"completion": {
		"gpt-5":                        0.010000,
		"gpt-5-mini":                   0.002000,
		"gpt-5-nano":                   0.000400,
		"gpt-5-chat-latest":            0.010000,
		"gpt-5-codex":                  0.010000,
		"gpt-5-pro":                    0.120000,
		"gpt-4.1":                      0.008000,
		"gpt-4.1-mini":                 0.001600,
		"gpt-4.1-nano":                 0.000400,
		"gpt-4o":                       0.010000,
		"gpt-4o-2024-05-13":            0.015000,
		"gpt-4o-mini":                  0.000600,
		"gpt-realtime":                 0.016000,
		"gpt-realtime-mini":            0.002400,
		"gpt-4o-realtime-preview":      0.020000,
		"gpt-4o-mini-realtime-preview": 0.002400,
		"gpt-audio":                    0.010000,
		"gpt-audio-mini":               0.002400,
		"gpt-4o-audio-preview":         0.010000,
		"gpt-4o-mini-audio-preview":    0.000600,
		"o1":                           0.060000,
		"o1-pro":                       0.600000,
		"o3-pro":                       0.080000,
		"o3":                           0.008000,
		"o3-deep-research":             0.040000,
		"o4-mini":                      0.004400,
		"o4-mini-deep-research":        0.008000,
		"o3-mini":                      0.004400,
		"o1-mini":                      0.004400,
		"codex-mini-latest":            0.006000,
		"gpt-4o-mini-search-preview":   0.000600,
		"gpt-4o-search-preview":        0.010000,
		"computer-use-preview":         0.012000,
		"chatgpt-4o-latest":            0.015000,
		"gpt-4-turbo-2024-04-09":       0.030000,
		"gpt-4-0125-preview":           0.030000,
		"gpt-4-1106-preview":           0.030000,
		"gpt-4-1106-vision-preview":    0.030000,
		"gpt-4-0613":                   0.060000,
		"gpt-4-0314":                   0.060000,
		"gpt-4-32k":                    0.120000,
		"gpt-3.5-turbo":                0.001500,
		"gpt-3.5-turbo-0125":           0.001500,
		"gpt-3.5-turbo-1106":           0.002000,
		"gpt-3.5-turbo-0613":           0.002000,
		"gpt-3.5-0301":                 0.002000,
		"gpt-3.5-turbo-instruct":       0.002000,
		"gpt-3.5-turbo-16k-0613":       0.004000,
		"davinci-002":                  0.002000,
		"babbage-002":                  0.000400,
	},
	"images": {
		"dall-e-2":      0.02,
		"dall-e-2-256":  0.016,
		"dall-e-2-512":  0.018,
		"dall-e-2-1024": 0.02,

		"dall-e-3":               0.04,
		"dall-e-3-1024-standart": 0.04,
		"dall-e-3-1792-standart": 0.08,
		"dall-e-3-1024-hd":       0.08,
		"dall-e-3-1792-hd":       0.12,
	},
}

var OpenAiPerThousandCallsToolCost = map[string]float64{
	"web_search":                   10.0,
	"web_search_preview":           25.0,
	"web_search_preview_reasoning": 10.0,
}

var AllowedTools = []string{
	"web_search",
	"web_search_preview",
}

type tokenCounter interface {
	Count(model string, input string) (int, error)
}

type CostEstimator struct {
	tokenCostMap map[string]map[string]float64
	tc           tokenCounter
}

func NewCostEstimator(m map[string]map[string]float64, tc tokenCounter) *CostEstimator {
	return &CostEstimator{
		tokenCostMap: m,
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

	cost, ok := costMap[useFinetuneModel(model)]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	tksInFloat := float64(tks)
	return tksInFloat / 1000 * cost, nil
}

func (ce *CostEstimator) EstimateEmbeddingsInputCost(model string, tks int) (float64, error) {
	costMap, ok := ce.tokenCostMap["embeddings"]
	if !ok {
		return 0, errors.New("embeddings token cost is not provided")

	}

	cost, ok := costMap[model]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}

	tksInFloat := float64(tks)
	return tksInFloat / 1000 * cost, nil
}

func (ce *CostEstimator) EstimateCompletionCost(model string, tks int) (float64, error) {
	costMap, ok := ce.tokenCostMap["completion"]
	if !ok {
		return 0, errors.New("prompt token cost is not provided")
	}

	cost, ok := costMap[useFinetuneModel(model)]
	if !ok {
		return 0, errors.New("model is not present in the cost map provided")
	}

	tksInFloat := float64(tks)
	return tksInFloat / 1000 * cost, nil
}

func (ce *CostEstimator) EstimateChatCompletionPromptTokenCounts(model string, r *goopenai.ChatCompletionRequest) (int, error) {
	tks, err := countTotalTokens(model, r, ce.tc)
	if err != nil {
		return 0, err
	}

	return tks, nil
}

func (ce *CostEstimator) EstimateChatCompletionPromptCostWithTokenCounts(r *goopenai.ChatCompletionRequest) (int, float64, error) {
	if len(r.Model) == 0 {
		return 0, 0, errors.New("model is not provided")
	}

	tks, err := countTotalTokens(r.Model, r, ce.tc)
	if err != nil {
		return 0, 0, err
	}

	cost, err := ce.EstimatePromptCost(r.Model, tks)
	if err != nil {
		return 0, 0, err
	}

	return tks, cost, nil
}

func (ce *CostEstimator) EstimateChatCompletionStreamCostWithTokenCounts(model string, content string) (int, float64, error) {
	if len(model) == 0 {
		return 0, 0, errors.New("model is not provided")
	}

	tks, err := ce.tc.Count(model, content)
	if err != nil {
		return 0, 0, err
	}

	cost, err := ce.EstimateCompletionCost(model, tks)
	if err != nil {
		return 0, 0, err
	}

	return tks, cost, nil
}

func (ce *CostEstimator) EstimateCompletionsRequestCostWithTokenCounts(model string, content any) (int, float64, error) {
	if len(model) == 0 {
		return 0, 0, errors.New("model is not provided")
	}

	input, err := util.ConvertAnyToStr(content)
	if err != nil {
		return 0, 0, err
	}

	tks, err := ce.tc.Count(model, input)
	if err != nil {
		return 0, 0, err
	}

	cost, err := ce.EstimatePromptCost(model, tks)
	if err != nil {
		return 0, 0, err
	}

	return tks, cost, nil
}

func (ce *CostEstimator) EstimateCompletionsStreamCostWithTokenCounts(model string, content string) (int, float64, error) {
	if len(model) == 0 {
		return 0, 0, errors.New("model is not provided")
	}

	tks, err := ce.tc.Count(model, content)
	if err != nil {
		return 0, 0, err
	}

	cost, err := ce.EstimateCompletionCost(model, tks)
	if err != nil {
		return 0, 0, err
	}

	return tks, cost, nil
}

func (ce *CostEstimator) EstimateImagesCost(model, quality, resolution string) (float64, error) {
	simpleRes, err := convertResToSimple(resolution)
	if err != nil {
		return 0, err
	}
	var normalizedModel string
	switch model {
	case "dall-e-2":
		normalizedModel, err = prepareDallE2Model(simpleRes, model)
		if err != nil {
			return 0, err
		}
	case "dall-e-3":
		normalizedModel, err = prepareDallE3Model(quality, simpleRes, model)
		if err != nil {
			return 0, err
		}
	default:
		return 0, errors.New("model is not present in the images cost map")
	}

	costMap, ok := ce.tokenCostMap["images"]
	if !ok {
		return 0, errors.New("images cost map is not provided")
	}
	cost, ok := costMap[normalizedModel]
	if !ok {
		return 0, errors.New("model is not present in the images cost map")
	}
	return cost, nil
}

var allowedDallE2Resolutions = []string{"256", "512", "1024"}
var allowedDallE3Resolutions = []string{"1024", "1792"}
var allowedDallE3Qualities = []string{"standart", "hd"}

func convertResToSimple(resolution string) (string, error) {
	if resolution == "" {
		return "", nil
	}
	if strings.Contains(resolution, "1792") {
		return "1792", nil
	}
	if strings.Contains(resolution, "1024") {
		return "1024", nil
	}
	if strings.Contains(resolution, "512") {
		return "512", nil
	}
	if strings.Contains(resolution, "256") {
		return "256", nil
	}
	return "", errors.New("resolution is not valid")
}

func prepareDallE2Model(resolution, model string) (string, error) {
	if resolution == "" {
		return model, nil
	}
	if slices.Contains(allowedDallE2Resolutions, resolution) {
		return fmt.Sprintf("%s-%s", model, resolution), nil
	}
	return "", errors.New("resolution is not valid")
}

func prepareDallE3Model(quality, resolution, model string) (string, error) {
	preparedQuality, err := prepareDallE3Quality(quality)
	if err != nil {
		return "", err
	}
	if resolution == "" && quality == "" {
		return model, nil
	}
	if resolution == "" {
		return fmt.Sprintf("%s-%s-%s", model, "1024", preparedQuality), nil
	}
	if slices.Contains(allowedDallE3Resolutions, resolution) {
		return fmt.Sprintf("%s-%s-%s", model, resolution, preparedQuality), nil
	}
	return "", errors.New("resolution is not valid")
}

func prepareDallE3Quality(quality string) (string, error) {
	if quality != "" && !slices.Contains(allowedDallE3Qualities, quality) {
		return "", errors.New("quality is not valid")
	}
	if quality == "" {
		return "standart", nil
	}
	return quality, nil
}

func (ce *CostEstimator) EstimateTranscriptionCost(secs float64, model string) (float64, error) {
	costMap, ok := ce.tokenCostMap["audio"]
	if !ok {
		return 0, errors.New("audio cost map is not provided")
	}

	cost, ok := costMap[model]
	if !ok {
		return 0, errors.New("model is not present in the audio cost map")
	}

	return math.Trunc(secs) / 60 * cost, nil
}

func (ce *CostEstimator) EstimateSpeechCost(input string, model string) (float64, error) {
	costMap, ok := ce.tokenCostMap["audio"]
	if !ok {
		return 0, errors.New("audio cost map is not provided")
	}

	cost, ok := costMap[model]
	if !ok {
		return 0, errors.New("model is not present in the audio cost map")
	}

	return float64(len(input)) / 1000 * cost, nil
}

func (ce *CostEstimator) EstimateFinetuningCost(num int, model string) (float64, error) {
	costMap, ok := ce.tokenCostMap["finetune"]
	if !ok {
		return 0, errors.New("audio cost map is not provided")
	}

	cost, ok := costMap[model]
	if !ok {
		return 0, errors.New("model is not present in the audio cost map")
	}

	return cost * float64(num), nil
}

func (ce *CostEstimator) EstimateEmbeddingsCost(r *goopenai.EmbeddingRequest) (float64, error) {
	if len(string(r.Model)) == 0 {
		return 0, errors.New("model is not provided")
	}

	input, err := util.ConvertAnyToStr(r.Input)
	if err != nil {
		return 0, err
	}

	tks, err := ce.tc.Count(string(r.Model), input)
	if err != nil {
		return 0, err
	}

	return ce.EstimateEmbeddingsInputCost(string(r.Model), tks)
}

func (ce *CostEstimator) EstimateResponseApiTotalCost(model string, usage responsesOpenai.ResponseUsage) (float64, error) {
	if len(model) == 0 {
		return 0, errors.New("model is not provided")
	}

	inputTokens := usage.InputTokens
	cachedInputTokens := usage.InputTokensDetails.CachedTokens
	outputTokens := usage.OutputTokens

	cachedInputCost, err := ce.estimateResponseApiTokensCost("cached-prompt", model, cachedInputTokens)
	if err != nil {
		cachedInputTokens = 0.0
	}
	inputCost, err := ce.estimateResponseApiTokensCost("prompt", model, inputTokens-cachedInputTokens)
	if err != nil {
		return 0.0, err
	}

	outputCost, err := ce.estimateResponseApiTokensCost("completion", model, outputTokens)

	return inputCost + cachedInputCost + outputCost, err
}

func (ce *CostEstimator) EstimateResponseApiToolCallsCost(tools []responsesOpenai.ToolUnion, model string) (float64, error) {
	if len(tools) == 0 {
		return 0, nil
	}
	totalCost := 0.0
	for _, tool := range tools {
		toolType := tool.Type
		cost, ok := OpenAiPerThousandCallsToolCost[extendedToolType(toolType, model)]
		if !ok {
			return 0, fmt.Errorf("tool type %s is not present in the tool cost map provided", toolType)
		}
		totalCost += cost
	}
	return totalCost / 1000, nil
}

var reasoningModelPrefix = []string{"gpt-5", "o1", "o2", "o3"}

func extendedToolType(toolType, model string) string {
	if toolType != "web_search_preview" {
		return toolType
	}
	if slices.ContainsFunc(reasoningModelPrefix, func(s string) bool { return strings.HasPrefix(model, s) }) {
		return "web_search_preview_reasoning"
	}
	return toolType
}

func (ce *CostEstimator) estimateResponseApiTokensCost(costMapKey, model string, tks int64) (float64, error) {
	costMap, ok := ce.tokenCostMap[costMapKey]
	if !ok {
		return 0, errors.New("cost map is not provided")
	}
	cost, ok := costMap[model]
	if !ok {
		return 0, fmt.Errorf("%s is not present in the cost map provided", model)
	}
	tksInFloat := float64(tks)
	return tksInFloat / 1000 * cost, nil
}

func countFunctionTokens(model string, r *goopenai.ChatCompletionRequest, tc tokenCounter) (int, error) {
	if len(r.Functions) == 0 {
		return 0, nil
	}

	defs := formatFunctionDefinitions(r)
	tks, err := tc.Count(model, defs)
	if err != nil {
		return 0, err
	}

	tks += 9
	return tks, nil
}

func formatFunctionDefinitions(r *goopenai.ChatCompletionRequest) string {
	lines := []string{
		"namespace functions {", "",
	}

	for _, f := range r.Functions {
		if len(f.Description) != 0 {
			lines = append(lines, fmt.Sprintf("// %s", f.Description))
		}

		if f.Parameters != nil {
			lines = append(lines, fmt.Sprintf("type %s = (_: {`", f.Name))

			params := &FuntionCallProp{}
			data, err := json.Marshal(f.Parameters)
			if err == nil {
				err := json.Unmarshal(data, params)
				if err == nil {
					lines = append(lines, formatObjectProperties(params, 0))
				}
			}

			lines = append(lines, "}) => any;")
		}

		if f.Parameters == nil {
			lines = append(lines, fmt.Sprintf("type %s = () => any;", f.Name))
		}

		lines = append(lines, "")
	}

	lines = append(lines, "} // namespace functions")
	return strings.Join(lines, "\n")
}

func countMessageTokens(model string, r *goopenai.ChatCompletionRequest, tc tokenCounter) (int, error) {
	if len(r.Messages) == 0 {
		return 0, nil
	}

	result := 0
	padded := false

	for _, msg := range r.Messages {
		content := msg.Content
		if msg.Role == "system" && !padded {
			content += "\n"
			padded = true
		}

		contentTks, err := tc.Count(model, content)
		if err != nil {
			return 0, err
		}

		roleTks, err := tc.Count(model, msg.Role)
		if err != nil {
			return 0, err
		}

		nameTks, err := tc.Count(model, msg.Name)
		if err != nil {
			return 0, err
		}

		result += contentTks
		result += roleTks
		result += nameTks

		result += 3
		if len(msg.Name) != 0 {
			result += 1
		}

		if msg.Role == "function" {
			result -= 2
		}

		if msg.FunctionCall != nil {
			result += 3
		}
	}

	return result, nil
}

func countTotalTokens(model string, r *goopenai.ChatCompletionRequest, tc tokenCounter) (int, error) {
	if r == nil {
		return 0, nil
	}

	tks := 3

	ftks, err := countFunctionTokens(model, r, tc)
	if err != nil {
		return 0, err
	}

	mtks, err := countMessageTokens(model, r, tc)
	if err != nil {
		return 0, err
	}

	systemExists := false
	for _, msg := range r.Messages {
		if msg.Role == "system" {
			systemExists = true
		}

	}

	if len(r.Functions) != 0 && systemExists {
		tks -= 4
	}

	return tks + ftks + mtks, err
}
