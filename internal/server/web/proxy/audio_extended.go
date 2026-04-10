package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

const (
	transcriptionsUrl = "https://api.openai.com/v1/audio/transcriptions"
	translationsUrl   = "https://api.openai.com/v1/audio/translations"
)

func processGPTTranscriptions(ginCtx *gin.Context, prod bool, client http.Client, e estimator, model string) {
}

func processGPTTranslations(ginCtx *gin.Context, prod bool, client http.Client, e estimator, model string) {
}

func processGPTAudio(ginCtx *gin.Context, prod bool, client http.Client, e estimator, model, url, handler string) {
	log := util.GetLogFromCtx(ginCtx)
	telemetry.Incr(fmt.Sprintf("bricksllm.proxy.get_%s_handler.requests", handler), nil, 1)

	if ginCtx.Request == nil {
		JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] request is empty")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ginCtx.GetDuration("requestTimeout"))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, ginCtx.Request.Method, url, ginCtx.Request.Body)
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
		readBytes, err := io.ReadAll(res.Body)
		if err != nil {
			logError(log, "error when reading openai http transcriptions response body", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to read openai response body")
			return
		}
		var cost float64 = 0
		resp := &openai.TranscriptionResponse{}
		telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.success", nil, 1)
		telemetry.Timing("bricksllm.proxy.get_transcriptions_handler.success_latency", dur, nil, 1)

		err = json.Unmarshal(readBytes, resp)
		if err != nil {
			logError(log, "error when unmarshalling openai http response api response body", prod, err)
		}

		if err == nil {
			cost, err = e.EstimateTranscriptionCost(0, model, &resp.Usage)
			if err != nil {
				telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.estimate_total_cost_error", nil, 1)
				logError(log, "error when estimating openai cost", prod, err)
			}
		}

		ginCtx.Set("costInUsd", cost)

		contentType := "application/json"
		bytesToSend := readBytes
		if ginCtx.PostForm("response_format") == "text" {
			contentType = "text/plain; charset=utf-8"
			bytesToSend = []byte(resp.Text + "\n")
		}

		ginCtx.Data(res.StatusCode, contentType, bytesToSend)
		return
	}

	if res.StatusCode != http.StatusOK {
		dur := time.Since(start)
		telemetry.Timing("bricksllm.proxy.get_transcriptions_handler.error_latency", dur, nil, 1)
		telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.error_response", nil, 1)

		readBytes, err := io.ReadAll(res.Body)
		if err != nil {
			logError(log, "error when reading openai transcriptions response body", prod, err)
			JSON(ginCtx, http.StatusInternalServerError, "[BricksLLM] failed to read openai transcriptions response body")
			return
		}

		errorRes := &goopenai.ErrorResponse{}
		err = json.Unmarshal(readBytes, errorRes)
		if err != nil {
			logError(log, "error when unmarshalling openai transcriptions error response body", prod, err)
		}

		logOpenAiError(log, prod, errorRes)

		ginCtx.Data(res.StatusCode, "application/json", readBytes)
		return
	}

	buffer := bufio.NewReader(res.Body)
	content := ""
	streamingResponse := [][]byte{}

	streamCost := 0.0

	defer func() {
		ginCtx.Set("content", content)
		ginCtx.Set("streaming_response", bytes.Join(streamingResponse, []byte{'\n'}))

		ginCtx.Set("costInUsd", streamCost)
	}()

	telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.streaming_response", nil, 1)
	ginCtx.Stream(func(w io.Writer) bool {
		raw, err := buffer.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return false
			}

			if errors.Is(err, context.DeadlineExceeded) {
				telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.context_deadline_exceeded_error", nil, 1)
				logError(log, "context deadline exceeded when reading bytes from openai transcriptions response", prod, err)

				return false
			}

			telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.read_bytes_error", nil, 1)
			logError(log, "error when reading bytes from openai transcriptions response", prod, err)

			apiErr := &goopenai.ErrorResponse{
				Error: &goopenai.APIError{
					Type:    "bricksllm_error",
					Message: err.Error(),
				},
			}

			errBytes, err := json.Marshal(apiErr)
			if err != nil {
				telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.json_marshal_error", nil, 1)
				logError(log, "error when marshalling bytes for openai streaming transcriptions error response", prod, err)
				return false
			}

			ginCtx.SSEvent("", string(errBytes))
			ginCtx.SSEvent("", " [DONE]")
			return false
		}
		streamingResponse = append(streamingResponse, raw)

		noSpaceLine := bytes.TrimSpace(raw)
		if !bytes.HasPrefix(noSpaceLine, headerData) {
			return true
		}

		noPrefixLine := bytes.TrimPrefix(noSpaceLine, headerData)
		noPrefixLine = bytes.TrimSpace(noPrefixLine)
		ginCtx.SSEvent("", " "+string(noPrefixLine))

		if string(noPrefixLine) == "[DONE]" {
			return false
		}
		chunk := &openai.TranscriptionStreamChunk{}
		err = json.Unmarshal(noPrefixLine, chunk)
		if err != nil {
			telemetry.Incr("bricksllm.proxy.get_transcriptions_handler.completion_response_unmarshall_error", nil, 1)
			logError(log, "error when unmarshalling openai transcriptions stream response", prod, err)
		}
		if err == nil {
			textDelta := chunk.GetText()
			if len(textDelta) > 0 {
				content += textDelta
			}
			if chunk.IsDone() {
				content = chunk.GetText()
				streamCost, err = e.EstimateTranscriptionCost(0, model, &chunk.Usage)
			}
		}
		return true
	})
	telemetry.Timing("bricksllm.proxy.get_transcriptions_handler.streaming_latency", time.Since(start), nil, 1)
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
