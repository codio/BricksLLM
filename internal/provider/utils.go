package provider

import (
	"strings"
)

type ModelContextLenght struct {
	Model  string
	Tokens int64
}

var modelWithLengthCtx = []ModelContextLenght{
	// open ai
	ModelContextLenght{Model: "gpt-6-astra", Tokens: 272000},
	ModelContextLenght{Model: "gpt-6.1-sol", Tokens: 272000},
	ModelContextLenght{Model: "gpt-6-luna", Tokens: 272000},
	ModelContextLenght{Model: "gpt-5.6-sol", Tokens: 272000},
	ModelContextLenght{Model: "gpt-5.5", Tokens: 272000},
	ModelContextLenght{Model: "gpt-5.5-pro", Tokens: 272000},
	ModelContextLenght{Model: "gpt-5.4", Tokens: 272000},
	ModelContextLenght{Model: "gpt-5.4-pro", Tokens: 272000},

	// anthropic
	ModelContextLenght{Model: "claude-haiku-5-5", Tokens: 100000},
}

func ModelWithContextLength(model string, tokens int64) string {
	trueModel := strings.TrimSpace(model)
	for _, m := range modelWithLengthCtx {
		if trueModel == m.Model {
			return trueModel + contextLengthSuffixByTokens(tokens, m.Tokens)
		}
	}
	return trueModel
}

func contextLengthSuffixByTokens(tokens, threshold int64) string {
	if tokens >= threshold {
		return "~long"
	}
	return "~short"
}
