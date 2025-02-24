package xcustom

import (
	"fmt"
	"net/http"
	"regexp"

	"strings"
)

type XCustomSettings struct {
	Apikey       string `json:"apikey"`
	Endpoint     string `json:"endpoint"`
	AuthLocation string `json:"authLocation"`
	AuthTemplate string `json:"authTemplate"`
}

type AuthLocation string

var AuthLocations = struct {
	Header AuthLocation
	Query  AuthLocation
}{
	Header: AuthLocation("header"),
	Query:  AuthLocation("query"),
}

type XCustomAuth struct {
	Apikey   string
	Location AuthLocation
	Target   string
	Mask     string
}

const XProviderIdParam = "x_provider_id"

func IsXCustomRequest(req *http.Request) bool {
	return strings.HasPrefix(req.URL.RequestURI(), "/api/providers/xCustom/")
}

func GetXCustomAuth(req *http.Request, xSettings *XCustomSettings) (*XCustomAuth, error) {
	authLocation := getAuthLocation(xSettings.AuthLocation)
	var templateSeparator string
	switch authLocation {
	case AuthLocations.Header:
		templateSeparator = ":"
	case AuthLocations.Query:
		templateSeparator = "="
	default:
		return nil, fmt.Errorf("unknown auth location: %s", authLocation)
	}
	templateArr := strings.Split(xSettings.AuthTemplate, templateSeparator)
	if len(templateArr) != 2 {
		return nil, fmt.Errorf("invalid auth template: %s", xSettings.AuthTemplate)
	}
	target := strings.TrimSpace(templateArr[0])
	mask := strings.TrimSpace(templateArr[1])
	var reqAuthStr string
	switch authLocation {
	case AuthLocations.Header:
		reqAuthStr = req.Header.Get(target)
	case AuthLocations.Query:
		reqAuthStr = req.URL.Query().Get(target)
	default:
		return nil, fmt.Errorf("unknown auth location: %s", authLocation)
	}
	regexStr := strings.Replace(mask, "{{apikey}}", "(?P<key>.*)", -1)
	regex := regexp.MustCompile(regexStr)
	matches := regex.FindStringSubmatch(reqAuthStr)
	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid auth template: %s", xSettings.AuthTemplate)
	}
	key := strings.TrimSpace(matches[1])

	return &XCustomAuth{
		Apikey:   key,
		Location: authLocation,
		Target:   target,
		Mask:     mask,
	}, nil
}

func getAuthLocation(raw string) AuthLocation {
	switch raw {
	case "header":
		return AuthLocations.Header
	case "query":
		return AuthLocations.Query
	default:
		return AuthLocations.Header
	}
}
