package openai

import "strconv"

type ResponseRequest struct {
	Background         *bool                      `json:"background,omitzero"`
	Conversation       *any                       `json:"conversation,omitzero"`
	Include            []string                   `json:"include,omitzero"`
	Input              *any                       `json:"input,omitzero"`
	Instructions       *string                    `json:"instructions,omitzero"`
	MaxOutputTokens    *int                       `json:"max_output_tokens,omitzero"`
	MaxToolCalls       *int                       `json:"max_tool_calls,omitzero"`
	Metadata           *map[string]string         `json:"metadata,omitzero"`
	Model              *string                    `json:"model,omitzero"`
	ParallelToolCalls  *bool                      `json:"parallel_tool_calls,omitzero"`
	PreviousResponseId *string                    `json:"previous_response_id,omitzero"`
	Prompt             *any                       `json:"prompt,omitzero"`
	PromptCacheKey     *string                    `json:"prompt_cache_key,omitzero"`
	Reasoning          *any                       `json:"reasoning,omitzero"`
	SafetyIdentifier   *string                    `json:"safety_identifier,omitzero"`
	ServiceTier        *string                    `json:"service_tier,omitzero"`
	Store              *bool                      `json:"store,omitzero"`
	Stream             *bool                      `json:"stream,omitzero"`
	StreamOptions      *any                       `json:"stream_options,omitzero"`
	Temperature        *float32                   `json:"temperature,omitzero"`
	Text               *any                       `json:"text,omitzero"`
	ToolChoice         *any                       `json:"tool_choice,omitzero"`
	Tools              []ResponseRequestToolUnion `json:"tools,omitzero"`
	TopLogprobs        *int                       `json:"top_logprobs,omitzero"`
	TopP               *float32                   `json:"top_p,omitzero"`
	Truncation         *string                    `json:"truncation,omitzero"`
	//User            *string  `json:"user,omitzero"` //Deprecated
}

type ResponseRequestToolContainer struct {
	Type string `json:"type"`
	// memory_limit
	MemoryLimit *string `json:"memory_limit,omitzero"`
}

func (c *ResponseRequestToolContainer) GetMemoryLimit() string {
	if c.MemoryLimit != nil {
		return *c.MemoryLimit
	}
	return "1g"
}

type ResponseRequestToolUnion struct {
	Type      string `json:"type"`
	Container any    `json:"container"`
}

func (u *ResponseRequestToolUnion) GetContainerAsResponseRequestToolContainer() *ResponseRequestToolContainer {
	if container, ok := u.Container.(map[string]interface{}); ok {
		cType := "auto"
		rawType, exists := container["type"]
		if !exists {
			cType = "auto"
		}
		if typeStr, ok := rawType.(string); ok {
			cType = typeStr
		}
		toolContainer := &ResponseRequestToolContainer{
			Type:        cType,
			MemoryLimit: nil,
		}

		if memoryLimit, exists := container["memory_limit"]; exists {
			if memoryLimitStr, ok := memoryLimit.(string); ok {
				toolContainer.MemoryLimit = &memoryLimitStr
			}
		}
		return toolContainer
	}
	return nil
}

type ImageResponseUsage struct {
	TotalTokens        int                             `json:"total_tokens,omitempty"`
	InputTokens        int                             `json:"input_tokens,omitempty"`
	OutputTokens       int                             `json:"output_tokens,omitempty"`
	InputTokensDetails ImageResponseInputTokensDetails `json:"input_tokens_details,omitempty"`
}

type ImageResponseInputTokensDetails struct {
	TextTokens  int `json:"text_tokens,omitempty"`
	ImageTokens int `json:"image_tokens,omitempty"`
}
type ImageResponseMetadata struct {
	Quality string             `json:"quality,omitempty"`
	Size    string             `json:"size,omitempty"`
	Usage   ImageResponseUsage `json:"usage,omitempty"`
}

type VideoResponseMetadata struct {
	Model   string `json:"model,omitempty"`
	Size    string `json:"size,omitempty"`
	Seconds string `json:"seconds,omitempty"`
}

func (v *VideoResponseMetadata) GetSecondsAsFloat() (float64, error) {
	if v.Seconds == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(v.Seconds, 64)
}

type TranscriptionResponseUsageInputTokenDetails struct {
	TextTokens  int `json:"text_tokens,omitempty"`
	AudioTokens int `json:"audio_tokens,omitempty"`
}
type TranscriptionResponseUsage struct {
	Type              string                                      `json:"type"`
	TotalTokens       int                                         `json:"total_tokens,omitempty"`
	InputTokens       int                                         `json:"input_tokens,omitempty"`
	InputTokenDetails TranscriptionResponseUsageInputTokenDetails `json:"input_token_details,omitempty"`
	OutputTokens      int                                         `json:"output_tokens,omitempty"`
}
type TranscriptionResponse struct {
	Text  string                     `json:"text,omitempty"`
	Usage TranscriptionResponseUsage `json:"usage,omitempty"`
}

type TranscriptionStreamChunk struct {
	Type  string                     `json:"type"`
	Delta string                     `json:"delta,omitempty"`
	Text  string                     `json:"text,omitempty"`
	Usage TranscriptionResponseUsage `json:"usage,omitempty"`
}

func (c *TranscriptionStreamChunk) IsDone() bool {
	return c.Type == "transcript.text.done"
}

func (c *TranscriptionStreamChunk) IsDelta() bool {
	return c.Type == "transcript.text.delta"
}

func (c *TranscriptionStreamChunk) IsSegment() bool {
	return c.Type == "transcript.text.segment"
}

func (c *TranscriptionStreamChunk) GetText() string {
	if c.IsDelta() {
		return c.Delta
	}
	return c.Text
}

type VideoRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size"`
}
