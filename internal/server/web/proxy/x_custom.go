package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/bricks-cloud/bricksllm/internal/provider/xcustom"
	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// xCustomMaxCapturedResponseBytes caps how much of a response body gets
// buffered for later processing (c.Set). Responses larger than this are
// still proxied in full, they're just not captured.
const xCustomMaxCapturedResponseBytes = 5 * 1024 * 1024 // 5MB

// xCustomCapturingBody wraps a response body so its bytes keep flowing to
// the client exactly as they arrive (no buffering delay, streaming stays
// real-time), while also being copied into an in-memory buffer for later
// use. onClose runs once the upstream body has been fully read/closed.
type xCustomCapturingBody struct {
	io.ReadCloser
	buf      bytes.Buffer
	exceeded bool
	// failed is set when the upstream body read ends in an error other than
	// io.EOF (aborted/truncated response), so we don't capture partial data
	// as if it were the complete response.
	failed  bool
	onClose func(data []byte, exceeded bool)
}

func (b *xCustomCapturingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 && !b.exceeded {
		if b.buf.Len()+n > xCustomMaxCapturedResponseBytes {
			b.exceeded = true
			// drop the reference so the already-buffered bytes can be
			// garbage collected instead of being held for the rest of
			// a possibly long-lived stream.
			b.buf = bytes.Buffer{}
		} else {
			b.buf.Write(p[:n])
		}
	}

	if err != nil && err != io.EOF {
		b.failed = true
	}

	return n, err
}

func (b *xCustomCapturingBody) Close() error {
	err := b.ReadCloser.Close()

	if b.onClose != nil && !b.failed {
		b.onClose(b.buf.Bytes(), b.exceeded)
	}

	return err
}

func getXCustomHandler(prod bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := util.GetLogFromCtx(c)
		telemetry.Incr("bricksllm.proxy.get_x_custom_handler.requests", nil, 1)

		if c == nil || c.Request == nil {
			JSON(c, http.StatusInternalServerError, "[BricksLLM] context is empty")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.GetDuration("requestTimeout"))
		defer cancel()

		providerId := c.Param(xcustom.XProviderIdParam)
		rawProviderSettings, exists := c.Get("settings")
		if !exists {
			logError(log, "error provider setting", prod, errors.New("provider setting not found"))
			c.JSON(http.StatusInternalServerError, "[BricksLLM] no settings found")
			return
		}
		settings, ok := rawProviderSettings.([]*provider.Setting)
		if !ok {
			logError(log, "error provider setting", prod, errors.New("incorrect setting"))
			c.JSON(http.StatusInternalServerError, "[BricksLLM] incorrect provider setting")
			return
		}
		var providerSetting *provider.Setting
		for _, setting := range settings {
			if setting.Id == providerId {
				providerSetting = setting
			}
		}
		if providerSetting == nil {
			logError(log, "error provider setting", prod, errors.New("provider setting not found"))
			c.JSON(http.StatusInternalServerError, "[BricksLLM] no settings found")
			return
		}
		wildcard := c.Param("wildcard")
		endpoint := strings.TrimSuffix(providerSetting.GetParam("endpoint"), "/")
		targetUrl := fmt.Sprintf("%s%s", endpoint, wildcard)
		target, e := url.Parse(targetUrl)
		if e != nil {
			logError(log, "error parsing target url", prod, e)
			c.JSON(http.StatusInternalServerError, "[BricksLLM] invalid endpoint")
			return
		}

		proxy := &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(target)
				r.Out.URL.Path, r.Out.URL.RawPath = target.Path, target.RawPath
				r.Out.WithContext(ctx)

				// Let the transport negotiate and transparently decompress
				// the upstream response itself; otherwise a forwarded
				// client Accept-Encoding disables that and ModifyResponse
				// would capture raw compressed bytes instead of text.
				r.Out.Header.Del("Accept-Encoding")
			},
			ModifyResponse: func(res *http.Response) error {
				if res.Body == nil || res.StatusCode == http.StatusSwitchingProtocols {
					// A 101 response's Body is an io.ReadWriteCloser used
					// for bidirectional upgrade proxying (e.g. WebSocket);
					// wrapping it would strip that and break the upgrade.
					return nil
				}

				mediaType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
				isStreaming := mediaType == "text/event-stream"

				res.Body = &xCustomCapturingBody{
					ReadCloser: res.Body,
					onClose: func(data []byte, exceeded bool) {
						if exceeded || len(data) == 0 {
							return
						}

						if isStreaming {
							c.Set("content", string(data))
							c.Set("streaming_response", data)
							return
						}

						c.Set("response", data)
					},
				}

				return nil
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
