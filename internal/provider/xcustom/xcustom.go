package xcustom

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type XCustomSettings struct {
	Apikey   string `json:"apikey"`
	Endpoint string `json:"endpoint"`
	Header   string `json:"header"`
	MaskAuth string `json:"maskAuth"`
}

const XProviderIdParam = "x_provider_id"

func IsXCustomRequest(req *http.Request) bool {
	return strings.HasPrefix(req.URL.RequestURI(), "/api/providers/xCustom/")
}

func ExtractBricksKey(header, mask string) (key string, err error) {
	regexStr := strings.Replace(mask, "{{apikey}}", "(?P<key>.*)", -1)
	regex := regexp.MustCompile(regexStr)
	matches := regex.FindStringSubmatch(header)
	if len(matches) < 2 {
		err = fmt.Errorf("unable to extract bricks key")
		return
	}
	key = strings.TrimSpace(matches[1])
	return
}
