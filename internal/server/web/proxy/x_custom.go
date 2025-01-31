package proxy

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/bricks-cloud/bricksllm/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

type XCustomSettings struct {
	Apikey   string `json:"apikey"`
	Endpoint string `json:"endpoint"`
	Header   string `json:"header"`
	MaskAuth string `json:"maskAuth"`
}

func getXCustomHandler(prod bool, client http.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		logWithCid := util.GetLogFromCtx(c)
		ctx, cancel := context.WithTimeout(context.Background(), c.GetDuration("requestTimeout"))
		defer cancel()

		providerId := c.Param("x_provider_id")
		rawProviderSettings, exists := c.Get("settings")
		if !exists {
			//fmt.Println("[BricksLLM] no settings found")
			c.JSON(http.StatusInternalServerError, "[BricksLLM] no settings found")
			return
		}
		settings, ok := rawProviderSettings.([]*provider.Setting)
		if !ok {
			//fmt.Println("[BricksLLM] no settings found")
			c.JSON(http.StatusInternalServerError, "[BricksLLM] no settings found")
			return
		}
		var providerSetting *provider.Setting
		for _, setting := range settings {
			if setting.Id == providerId {
				providerSetting = setting
			}
		}
		if providerSetting == nil {
			c.JSON(http.StatusInternalServerError, "[BricksLLM] no settings found")
			return
		}

		var setting *XCustomSettings
		err := mapstructure.Decode(providerSetting.Setting, &setting)
		if err != nil {
			//logError(logWithCid, "error when unmarshalling settings", prod, err)
			c.JSON(http.StatusInternalServerError, "[BricksLLM] no settings found")
			return
		}
		authHeaderVal := strings.Replace(setting.MaskAuth, "{{apikey}}", setting.Apikey, -1)
		wildcard := c.Param("wildcard")
		targetUrl := fmt.Sprintf("%s%s", setting.Endpoint, wildcard)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logError(logWithCid, "error when reading request body", prod, err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, c.Request.Method, targetUrl, io.NopCloser(bytes.NewReader(body)))
		if err != nil {
			logError(logWithCid, "error when creating custom provider http request", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] failed to create custom provider http request")
			return
		}

		copyHttpHeaders(c.Request, req, c.GetBool("removeUserAgent"))
		// remove connection ???
		req.Header.Del("Connection")
		req.Header.Del("Authorization")
		req.Header.Del("Api-Key")

		req.Header.Set(setting.Header, authHeaderVal)

		dumpRequest(req)

		res, err := client.Do(req)
		dumpResponse(res)
		if err != nil {
			//telemetry.Incr("bricksllm.proxy.get_custom_provider_handler.http_client_error", tags, 1)
			//logError(logWithCid, "error when sending custom provider request", prod, err)
			JSON(c, http.StatusInternalServerError, "[BricksLLM] failed to send custom provider request")
			return
		}
		defer res.Body.Close()

		responseBody, err := io.ReadAll(res.Body)
		if err != nil {
			fmt.Printf("[BricksLLM] failed to read custom provider response body")
		}
		fmt.Println(string(responseBody))
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func dumpRequest(r *http.Request) {
	if r == nil {
		fmt.Println("[BricksLLM] dumpRequest called with nil request")
		return
	}
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		fmt.Println("error dumping request", err)
		return
	}
	fmt.Println("-----------DUMP REQUEST----------")
	fmt.Println(string(dump))
	fmt.Println("===========DUMP REQUEST =============")
}

func dumpResponse(r *http.Response) {
	if r == nil {
		fmt.Println("[BricksLLM] dumpResponse called with nil request")
		return
	}
	dump, err := httputil.DumpResponse(r, true)
	if err != nil {
		fmt.Println("error dumping response", err)
		return
	}
	fmt.Println("-----------DUMP RESPONSE ========---")
	fmt.Println(string(dump))
	fmt.Println("============DUMP RESPONSE ============")
}
