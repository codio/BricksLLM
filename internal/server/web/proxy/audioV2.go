package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/provider/openai"
	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"
	goopenai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

const transcriptionsUrl = "https://api.openai.com/v1/audio/transcriptions"

func processGPTTranscriptions(ginCtx *gin.Context, prod bool, client http.Client, e estimator, model string) {
	log := util.GetLogFromCtx(ginCtx)
	telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.requests", nil, 1)

	if ginCtx == nil || ginCtx.Request == nil {
		JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] context is empty")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ginCtx.GetDuration("requestTimeout"))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, ginCtx.Request.Method, transcriptionsUrl, ginCtx.Request.Body)
	if err != nil {
		logError(log, "error when creating transcriptions openai http request", prod, err)
		JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to create openai transcriptions http request")
		return
	}

	copyHttpHeaders(ginCtx.Request, req, ginCtx.GetBool("removeUserAgent"))

	isStreaming := ginCtx.PostForm("stream") == "True" || ginCtx.PostForm("stream") == "true"

	if isStreaming {
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	}

	if !isStreaming {
		modifyGPTTranscriptionsRequest(ginCtx, prod, log, req)
	}

	start := time.Now()
	res, err := client.Do(req)
	if err != nil {
		telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.http_client_error", nil, 1)

		logError(log, "error when sending transcriptions request to openai", prod, err)
		JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to send transcriptions request to openai")
		return
	}

	defer res.Body.Close()

	for name, values := range res.Header {
		for _, value := range values {
			ginCtx.Header(name, value)
		}
	}

	if res.StatusCode == http.StatusOK && !isStreaming {
		dur := time.Since(start)
		telemetry.Timing("bricksllm.proxy.get_transcriptions_handler.latency", dur, nil, 1)
		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			logError(log, "error when reading openai http transcriptions response body", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to read openai response body")
			return
		}
		var cost float64 = 0
		resp := &openai.TranscriptionResponse{}
		telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.success", nil, 1)
		telemetry.Timing("bricksllm.proxy.get_transcriptions_handler.success_latency", dur, nil, 1)

		err = json.Unmarshal(bytes, resp)
		if err != nil {
			logError(log, "error when unmarshalling openai http response api response body", prod, err)
		}

		if err == nil {
			// estimate
		}

		ginCtx.Set("costInUsd", cost)

		contentType := "application/json"
		if ginCtx.PostForm("response_format") == "text" {
			contentType = "text/plain; charset=utf-8"
		}

		ginCtx.Data(res.StatusCode, contentType, bytes)
		return
	}

	if res.StatusCode != http.StatusOK {
		dur := time.Since(start)
		telemetry.Timing("bricksllm.proxy.get_transcriptions_handler.error_latency", dur, nil, 1)
		telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.error_response", nil, 1)

		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			logError(log, "error when reading openai transcriptions response body", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to read openai transcriptions response body")
			return
		}

		errorRes := &goopenai.ErrorResponse{}
		err = json.Unmarshal(bytes, errorRes)
		if err != nil {
			logError(log, "error when unmarshalling openai transcriptions error response body", prod, err)
		}

		logOpenAiError(log, prod, errorRes)

		ginCtx.Data(res.StatusCode, "application/json", bytes)
		return
	}
}

func modifyGPTTranscriptionsRequest(c *gin.Context, prod bool, log *zap.Logger, req *http.Request) {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	defer writer.Close()

	responseFormat := c.PostForm("response_format")
	if responseFormat == "text" {
		responseFormat = "json"
	}

	err := writePostFields(c, writer, map[string]string{
		"response_format": responseFormat,
	})
	if err != nil {
		telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.write_field_to_buffer_error", nil, 1)
		logError(log, "error when writing field to buffer", prod, err)
		JSON(c, http.StatusInternalServerError, "[BricksLLM] cannot write field to buffer")
		return
	}

	var form TransriptionForm
	c.ShouldBind(&form)

	if form.File != nil {
		fieldWriter, err := writer.CreateFormFile("file", form.File.Filename)
		if err != nil {
			telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.create_transcription_file_error", nil, 1)
			logError(log, "error when creating transcription file", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] cannot create transcription file")
			return
		}

		opened, err := form.File.Open()
		if err != nil {
			telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.open_transcription_file_error", nil, 1)
			logError(log, "error when openning transcription file", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] cannot open transcription file")
			return
		}

		_, err = io.Copy(fieldWriter, opened)
		if err != nil {
			telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.copy_transcription_file_error", nil, 1)
			logError(log, "error when copying transcription file", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] cannot copy transcription file")
			return
		}
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Body = io.NopCloser(&b)
}

func writePostFields(c *gin.Context, writer *multipart.Writer, overWrites map[string]string) error {
	for k, v := range c.Request.PostForm {
		if len(v) == 0 {
			continue
		}
		val := v[0]
		if len(overWrites) != 0 {
			if ow := overWrites[k]; len(ow) != 0 {
				val = ow
			}
		}
		if len(val) == 0 {
			continue
		}
		err := writer.WriteField(k, val)
		if err != nil {
			return err
		}
	}
	return nil
}
