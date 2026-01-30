package event

import (
	"github.com/bricks-cloud/bricksllm/internal/key"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/bricks-cloud/bricksllm/internal/provider/custom"
	"github.com/bricks-cloud/bricksllm/internal/provider/openai"
)

type EventWithRequestAndContent struct {
	Event                 *Event
	IsEmbeddingsRequest   bool
	RouteConfig           *custom.RouteConfig
	Request               interface{}
	Content               string
	Response              interface{}
	Key                   *key.ResponseKey
	CostMap               *provider.CostMap
	ImageResponseMetadata *openai.ImageResponseMetadata
}
