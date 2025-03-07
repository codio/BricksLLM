package proxy

import (
	"context"
	"errors"
	"fmt"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/bricks-cloud/bricksllm/internal/provider/xcustom"
	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

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
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
