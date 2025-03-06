package xcustom

import (
	"fmt"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"net/http"
	"regexp"

	"strings"
)

var XCustomSettingFields = struct {
	ApiKey       string
	Endpoint     string
	AuthLocation string
	AuthTemplate string
	AuthTarget   string
	AuthMask     string
}{
	ApiKey:       "apikey",
	Endpoint:     "endpoint",
	AuthLocation: "authLocation",
	AuthTemplate: "authTemplate",
	AuthTarget:   "authTarget",
	AuthMask:     "authMask",
}

type AuthLocation string

var AuthLocations = struct {
	Header  AuthLocation
	Query   AuthLocation
	Unknown AuthLocation
}{
	Header:  AuthLocation("header"),
	Query:   AuthLocation("query"),
	Unknown: AuthLocation("unknown"),
}

const XProviderIdParam = "x_provider_id"

func IsXCustomRequest(req *http.Request) bool {
	return strings.HasPrefix(req.URL.RequestURI(), "/api/providers/xCustom/")
}

func AdvancedXCustomSetting(src map[string]string) (map[string]string, error) {
	rawLocation := src[XCustomSettingFields.AuthLocation]
	location := GetAuthLocation(rawLocation)
	var templateSeparator string
	switch location {
	case AuthLocations.Header:
		templateSeparator = ":"
	case AuthLocations.Query:
		templateSeparator = "="
	default:
		return nil, fmt.Errorf("unknown auth location: %s", location)
	}
	templateArr := strings.Split(src[XCustomSettingFields.AuthTemplate], templateSeparator)
	if len(templateArr) != 2 {
		return nil, fmt.Errorf("invalid auth template: %s", src[XCustomSettingFields.AuthTemplate])
	}
	target := strings.TrimSpace(templateArr[0])
	mask := strings.TrimSpace(templateArr[1])
	return map[string]string{
		XCustomSettingFields.AuthTarget: target,
		XCustomSettingFields.AuthMask:   mask,
	}, nil
}

func ExtractApiKey(req *http.Request, pSetting *provider.Setting) (string, error) {
	location := GetAuthLocation(pSetting.GetParam(XCustomSettingFields.AuthLocation))
	target := strings.TrimSpace(pSetting.GetParam(XCustomSettingFields.AuthTarget))
	var reqAuthStr string
	switch location {
	case AuthLocations.Header:
		reqAuthStr = req.Header.Get(target)
	case AuthLocations.Query:
		reqAuthStr = req.URL.Query().Get(target)
	default:
		return "", fmt.Errorf("unknown auth location: %s", location)
	}
	mask := strings.TrimSpace(pSetting.GetParam(XCustomSettingFields.AuthMask))
	regexStr := strings.Replace(mask, "{{apikey}}", "(?P<key>.*)", -1)
	regex := regexp.MustCompile(regexStr)
	matches := regex.FindStringSubmatch(reqAuthStr)
	if len(matches) < 2 {
		return "", fmt.Errorf("error extracting apikey: %s", pSetting.Id)
	}
	return strings.TrimSpace(matches[1]), nil
}

func GetAuthLocation(raw string) AuthLocation {
	switch raw {
	case "header":
		return AuthLocations.Header
	case "query":
		return AuthLocations.Query
	default:
		return AuthLocations.Unknown
	}
}
