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
		dumpA, _ := httputil.DumpRequest(c.Request, true)
		c.Request.Header.Del("X-Amzn-Trace-Id")
		c.Request.Header.Del("X-Forwarded-For")
		c.Request.Header.Del("X-Forwarded-Port")
		c.Request.Header.Del("X-Forwarded-Proto")
		fmt.Println("=========HEADERS==============")
		fmt.Println(c.Request.Header)
		fmt.Println("=======dumpA===========")
		fmt.Println(string(dumpA))
		proxy := &httputil.ReverseProxy{
			Director: func(r *http.Request) {
				r.URL.Scheme = target.Scheme
				r.URL.Host = target.Host
				r.URL.Path, r.URL.RawPath = target.Path, target.RawPath
				r.RequestURI = target.RequestURI()
				r.Host = target.Host
				r = r.WithContext(ctx)

				r.Header.Del("X-Amzn-Trace-Id")
				r.Header.Del("X-Forwarded-For")
				r.Header.Del("X-Forwarded-Port")
				r.Header.Del("X-Forwarded-Proto")

				dumpB, _ := httputil.DumpRequest(r, true)
				fmt.Println("=======dumpB===========")
				fmt.Println(string(dumpB))
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
