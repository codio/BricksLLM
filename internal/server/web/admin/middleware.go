package admin

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/util"
	macverification "github.com/bricks-cloud/bricksllm/internal/util/mac-verification"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func getAdminLoggerMiddleware(log *zap.Logger, prefix string, prod bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := util.NewUuid()
		c.Set(util.STRING_CORRELATION_ID, cid)
		logWithCid := log.With(zap.String(util.STRING_CORRELATION_ID, cid))
		util.SetLogToCtx(c, logWithCid)

		start := time.Now()
		c.Next()
		latency := time.Since(start).Milliseconds()
		if !prod {
			logWithCid.Sugar().Infof("%s | %d | %s | %s | %dms", prefix, c.Writer.Status(), c.Request.Method, c.FullPath(), latency)
		}

		if prod {
			logWithCid.Info("request to admin management api",
				zap.Int("code", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", c.FullPath()),
				zap.Int64("latencyInMs", latency),
			)
		}
	}
}

func getAdminSignRequestMiddleware(prod bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := util.GetLogFromCtx(c)

		if !prod {
			c.Next()
			return
		}

		timestamp := c.GetHeader("X-Codio-Sign-Timestamp")
		token := c.GetHeader("X-Codio-Sign")
		provider := c.GetHeader("X-Codio-Provider")

		if len(token) == 0 || len(timestamp) == 0 || len(provider) == 0 {
			c.Status(403)
			c.Abort()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logError(log, "error when reading get events request body", prod, err)
			c.JSON(http.StatusInternalServerError, &ErrorResponse{
				Type:     "/errors/request-body-read",
				Title:    "get events request body reader error",
				Status:   http.StatusInternalServerError,
				Detail:   err.Error(),
				Instance: c.FullPath(),
			})
			return
		}
		data := fmt.Sprintf("%s%s%s", timestamp, body, provider)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if !macverification.VerifySign(provider, []byte(data), token) {
			c.Status(403)
			c.Abort()
			return
		}
		c.Next()
	}
}
