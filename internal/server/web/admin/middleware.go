package admin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func getAdminLoggerMiddleware(log *zap.Logger, prefix string, prod bool, adminPass string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(adminPass) != 0 && c.Request.Header.Get("X-API-KEY") != adminPass {
			c.Status(200)
			c.Abort()
			return
		}

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
				zap.Int64("lantecyInMs", latency),
			)
		}
	}
}

func getAdminSignRequestMiddleware(prod bool, xCodioSignSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := util.GetLogFromCtx(c)

		if !prod {
			c.Next()
			return
		}
		sign := c.Request.Header.Get("X-Codio-Sign")
		timestamp := c.Request.Header.Get("X-Codio-Sign-Timestamp")
		if len(sign) == 0 || len(timestamp) == 0 || len(xCodioSignSecret) == 0 {
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
		data := fmt.Sprintf("%s%s", timestamp, body)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if !validSign([]byte(data), []byte(xCodioSignSecret), sign) {
			c.Status(403)
			c.Abort()
			return
		}
		c.Next()
	}
}

func validSign(message, key []byte, messageSign string) bool {
	mac := hmac.New(sha1.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	expectedSign := base64.StdEncoding.EncodeToString(expectedMAC)
	return messageSign == expectedSign
}
