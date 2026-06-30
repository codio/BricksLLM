package anthropic

import (
	"encoding/json"
	"fmt"
)

type Metadata struct {
	UserId string `json:"user_id"`
}

type CompletionRequest struct {
	Model             string    `json:"model"`
	Prompt            string    `json:"prompt"`
	MaxTokensToSample int       `json:"max_tokens_to_sample"`
	StopSequences     []string  `json:"stop_sequences,omitempty"`
	Temperature       float32   `json:"temperature,omitempty"`
	TopP              int       `json:"top_p,omitempty"`
	TopK              int       `json:"top_k,omitempty"`
	Metadata          *Metadata `json:"metadata,omitempty"`
	Stream            bool      `json:"stream,omitempty"`
}

type FlexContent struct {
	Text string
	Raw  []interface{}
}

func (f *FlexContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		f.Text = s
		return nil
	}

	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	f.Raw = arr
	return nil
}

func (f *FlexContent) String() string {
	if f == nil {
		return ""
	}
	if f.Text != "" {
		return f.Text
	}
	if len(f.Raw) > 0 {
		return fmt.Sprintf("%v", f.Raw)
	}
	return ""
}

type Message struct {
	Content FlexContent `json:"content"`
	Role    string      `json:"role"`
}

type MessagesRequest struct {
	Model         string    `json:"model"`
	Messages      []Message `json:"messages"`
	MaxTokens     int       `json:"max_tokens"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
	Temperature   float32   `json:"temperature,omitempty"`
	TopP          int       `json:"top_p,omitempty"`
	TopK          int       `json:"top_k,omitempty"`
	Metadata      *Metadata `json:"metadata,omitempty"`
	Stream        bool      `json:"stream,omitempty"`
}

type CompletionResponse struct {
	Completion string `json:"completion"`
	StopReason string `json:"stop_reason"`
	Model      string `json:"model"`
}

type MessageResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type MessagesResponse struct {
	Id           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []MessageResponseContent `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence string                   `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
}

type MessagesStreamMessageStart struct {
	Message MessagesResponse `json:"message"`
}

type MessagesStreamMessageDelta struct {
	Delta struct {
		StopReason   string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence,omitempty"`
	} `json:"delta"`

	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type MessagesStreamMessageStop struct {
	Type string `json:"type"`
}

type MessagesStreamBlockStart struct {
	Index        int                    `json:"index"`
	ContentBlock MessageResponseContent `json:"content_block"`
}

type MessagesStreamBlockDelta struct {
	Index int                    `json:"index"`
	Delta MessageResponseContent `json:"delta"`
}

type MessagesStreamBlockStop struct {
	Index int `json:"index"`
}

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error *Error `json:"error"`
}
