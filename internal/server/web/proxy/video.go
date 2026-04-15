package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/provider/openai"
	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"
	goopenai "github.com/sashabaranov/go-openai"
)

func getVideoHandler(prod bool, client http.Client, e estimator) gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		log := util.GetLogFromCtx(ginCtx)
		telemetry.Incr("bricksllm.proxy.get_responses_handler.requests", nil, 1)

		if ginCtx == nil || ginCtx.Request == nil {
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] context is empty")
			return
		}

		ctx, cancel := context.WithTimeout(ginCtx.Request.Context(), ginCtx.GetDuration("requestTimeout"))
		defer cancel()

		videoURL, err := constructVideoURL(ginCtx.Request.URL.Path)
		if err != nil {
			logError(log, "failed to construct video URL", prod, err)
			JSON(ginCtx, http.StatusBadRequest, "[BricksLLM] invalid video request")
			return
		}

		req, err := http.NewRequestWithContext(ctx, ginCtx.Request.Method, videoURL, ginCtx.Request.Body)
		if err != nil {
			logError(log, "error when creating openai http request", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to create openai http request")
			return
		}

		copyHttpHeaders(ginCtx.Request, req, ginCtx.GetBool("removeUserAgent"))

		start := time.Now()
		res, err := client.Do(req)
		if err != nil {
			telemetry.Incr("bricksllm.proxy.get_video_handler.http_client_error", nil, 1)

			logError(log, "error when sending http request to openai", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to send http request to openai")
			return
		}
		defer res.Body.Close()

		for name, values := range res.Header {
			for _, value := range values {
				ginCtx.Header(name, value)
			}
		}

		if res.StatusCode != http.StatusOK {
			dur := time.Since(start)
			telemetry.Timing("bricksllm.proxy.get_video_handler.error_latency", dur, nil, 1)
			telemetry.Incr("bricksllm.proxy.get_video_handler.error_response", nil, 1)

			bytes, err2 := io.ReadAll(res.Body)
			if err2 != nil {
				logError(log, "error when reading openai http video response body", prod, err2)
				JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to read openai response body")
				return
			}

			errorRes := &goopenai.ErrorResponse{}
			err2 = json.Unmarshal(bytes, errorRes)
			if err2 != nil {
				logError(log, "error when unmarshalling openai video error response body", prod, err2)
			}

			logOpenAiError(log, prod, errorRes)

			ginCtx.Data(res.StatusCode, "application/json", bytes)
			return
		}

		dur := time.Since(start)
		telemetry.Timing("bricksllm.proxy.get_video_handler.latency", dur, nil, 1)

		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			logError(log, "error when reading openai http video response body", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to read openai response body")
			return
		}

		var cost float64 = 0
		respMetadata := &openai.VideoResponseMetadata{}
		telemetry.Incr("bricksllm.proxy.get_video_handler.success", nil, 1)
		telemetry.Timing("bricksllm.proxy.get_video_handler.success_latency", dur, nil, 1)

		err = json.Unmarshal(bytes, respMetadata)
		if err != nil {
			logError(log, "error when unmarshalling openai http video response body", prod, err)
		}

		isPaidRequest := ginCtx.Request.Method == http.MethodPost
		if err == nil && isPaidRequest {
			cost, err = e.EstimateVideoCost(respMetadata)
			if err != nil {
				telemetry.Incr("bricksllm.proxy.get_video_handler.estimate_cost_error", nil, 1)
				logError(log, "error when estimating video cost", prod, err)
			}
		}
		ginCtx.Set("costInUsd", cost)
		ginCtx.Data(res.StatusCode, res.Header.Get("Content-Type"), bytes)
		return
	}
}

func constructVideoURL(fullPath string) (string, error) {
	if fullPath == "" {
		return "", errors.New("empty full path")
	}
	if !strings.HasPrefix(fullPath, "/api/providers/openai") {
		return "", errors.New("invalid path prefix")
	}
	path := strings.TrimPrefix(fullPath, "/api/providers/openai")
	return "https://api.openai.com" + path, nil
}
