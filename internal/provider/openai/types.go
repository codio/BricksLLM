package openai

type ResponseRequest struct {
	Background         *bool              `json:"background,omitzero"`
	Conversation       *any               `json:"conversation,omitzero"`
	Include            []string           `json:"include,omitzero"`
	Input              *any               `json:"input,omitzero"`
	Instructions       *string            `json:"instructions,omitzero"`
	MaxOutputTokens    *int               `json:"max_output_tokens,omitzero"`
	MaxToolCalls       *int               `json:"max_tool_calls,omitzero"`
	Metadata           *map[string]string `json:"metadata,omitzero"`
	Model              *string            `json:"model,omitzero"`
	ParallelToolCalls  *bool              `json:"parallel_tool_calls,omitzero"`
	PreviousResponseId *string            `json:"previous_response_id,omitzero"`
	Prompt             *any               `json:"prompt,omitzero"`
	PromptCacheKey     *string            `json:"prompt_cache_key,omitzero"`
	Reasoning          *any               `json:"reasoning,omitzero"`
	SafetyIdentifier   *string            `json:"safety_identifier,omitzero"`
	ServiceTier        *string            `json:"service_tier,omitzero"`
	Store              *bool              `json:"store,omitzero"`
	Stream             *bool              `json:"stream,omitzero"`
	StreamOptions      *any               `json:"stream_options,omitzero"`
	Temperature        *float32           `json:"temperature,omitzero"`
	Text               *any               `json:"text,omitzero"`
	ToolChoice         *any               `json:"tool_choice,omitzero"`
	Tools              []any              `json:"tools,omitzero"`
	TopLogprobs        *int               `json:"top_logprobs,omitzero"`
	TopP               *float32           `json:"top_p,omitzero"`
	Truncation         *string            `json:"truncation,omitzero"`
	//User            *string  `json:"user,omitzero"` //Deprecated
}
