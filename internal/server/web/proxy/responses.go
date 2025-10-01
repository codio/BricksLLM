package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"

	goopenai "github.com/sashabaranov/go-openai"
)

func getResponsesHandler(prod, private bool, client http.Client, e estimator) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := util.GetLogFromCtx(c)
		telemetry.Incr("bricksllm.proxy.get_responses_handler.requests", nil, 1)

		if c == nil || c.Request == nil {
			JSON(c, http.StatusInternalServerError, "[BricksLLM] context is empty")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), c.GetDuration("requestTimeout"))
		defer cancel()

		wildcard := c.Param("wildcard")
		url := fmt.Sprintf("https://api.openai.com/v1/responses%s", wildcard)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, c.Request.Body)
		if err != nil {
			logError(log, "error when creating openai http request", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] failed to create openai http request")
			return
		}

		copyHttpHeaders(c.Request, req, c.GetBool("removeUserAgent"))

		// TODO
		isStreaming := c.GetBool("stream")
		//if isStreaming {
		//	req.Header.Set("Accept", "text/event-stream")
		//	req.Header.Set("Cache-Control", "no-cache")
		//	req.Header.Set("Connection", "keep-alive")
		//}

		start := time.Now()
		res, err := client.Do(req)
		if err != nil {
			telemetry.Incr("bricksllm.proxy.get_responses_handler.http_client_error", nil, 1)

			logError(log, "error when sending http request to openai", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] failed to send http request to openai")
			return
		}

		defer res.Body.Close()

		for name, values := range res.Header {
			for _, value := range values {
				c.Header(name, value)
			}
		}

		if res.StatusCode == http.StatusOK && !isStreaming {
			//	TODO: implement non-streaming logic here
		}

		if res.StatusCode != http.StatusOK {
			dur := time.Since(start)
			telemetry.Timing("bricksllm.proxy.get_responses_handler.error_latency", dur, nil, 1)
			telemetry.Incr("bricksllm.proxy.get_responses_handler.error_response", nil, 1)

			bytes, err := io.ReadAll(res.Body)
			if err != nil {
				logError(log, "error when reading openai http response api response body", prod, err)
				JSON(c, http.StatusInternalServerError, "[BricksLLM] failed to read openai response body")
				return
			}

			errorRes := &goopenai.ErrorResponse{}
			err = json.Unmarshal(bytes, errorRes)
			if err != nil {
				logError(log, "error when unmarshalling openai response api error response body", prod, err)
			}

			logOpenAiError(log, prod, errorRes)

			c.Data(res.StatusCode, "application/json", bytes)
			return
		}

		// TODO: implement the actual streaming logic here

	}
}
