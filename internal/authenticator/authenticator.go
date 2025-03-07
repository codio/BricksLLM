package auth

import (
	"errors"
	"fmt"
	"github.com/bricks-cloud/bricksllm/internal/provider/xcustom"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/hasher"
	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	metricname "github.com/bricks-cloud/bricksllm/internal/telemetry/metric_name"

	"github.com/bricks-cloud/bricksllm/internal/key"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	"github.com/bricks-cloud/bricksllm/internal/route"
)

type providerSettingsManager interface {
	GetSettingViaCache(id string) (*provider.Setting, error)
	GetSettingsViaCache(ids []string) ([]*provider.Setting, error)
}

type routesManager interface {
	GetRouteFromMemDb(path string) *route.Route
}

type keysCache interface {
	GetKeyViaCache(hash string) (*key.ResponseKey, error)
}

type keyStorage interface {
	GetKeyByHash(hash string) (*key.ResponseKey, error)
}

type Decryptor interface {
	Decrypt(input string, headers map[string]string) (string, error)
	Enabled() bool
}

type Authenticator struct {
	psm       providerSettingsManager
	kc        keysCache
	rm        routesManager
	ks        keyStorage
	decryptor Decryptor
}

func NewAuthenticator(psm providerSettingsManager, kc keysCache, rm routesManager, ks keyStorage, decryptor Decryptor) *Authenticator {
	return &Authenticator{
		psm:       psm,
		kc:        kc,
		rm:        rm,
		ks:        ks,
		decryptor: decryptor,
	}
}

func getApiKey(req *http.Request) (string, error) {
	list := []string{
		req.Header.Get("x-api-key"),
		req.Header.Get("api-key"),
	}

	split := strings.Split(req.Header.Get("Authorization"), " ")

	if len(split) >= 2 {
		list = append(list, split[1])
	}

	for _, key := range list {
		if len(key) != 0 {
			return key, nil
		}
	}

	return "", internal_errors.NewAuthError("api key not found in header")
}

func rewriteHttpAuthHeader(req *http.Request, setting *provider.Setting) error {
	uri := req.URL.RequestURI()
	if strings.HasPrefix(uri, "/api/routes") {
		return nil
	}

	apiKey := setting.GetParam("apikey")

	if strings.HasPrefix(uri, "/api/providers/vllm") {
		if len(apiKey) != 0 {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		}
		return nil
	}

	if len(apiKey) == 0 {
		if setting.Provider == "bedrock" {
			return nil
		}

		return errors.New("api key is empty in provider setting")
	}

	if strings.HasPrefix(uri, "/api/providers/anthropic") {
		req.Header.Set("x-api-key", apiKey)
		return nil
	}

	if strings.HasPrefix(uri, "/api/providers/azure") {
		req.Header.Set("api-key", apiKey)
		return nil
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	return nil
}

func (a *Authenticator) canKeyAccessCustomRoute(path string, keyId string) error {
	trimed := strings.TrimPrefix(path, "/api/routes")
	rc := a.rm.GetRouteFromMemDb(trimed)
	if rc == nil {
		return internal_errors.NewNotFoundError("route not found")
	}

	for _, kid := range rc.KeyIds {
		if kid == keyId {
			return nil
		}
	}

	return internal_errors.NewAuthError("not authorized")
}

func (a *Authenticator) getProviderSettingsThatCanAccessCustomRoute(path string, settings []*provider.Setting) []*provider.Setting {
	trimed := strings.TrimPrefix(path, "/api/routes")
	rc := a.rm.GetRouteFromMemDb(trimed)

	selected := []*provider.Setting{}
	if rc == nil {
		return []*provider.Setting{}
	}

	target := map[string]bool{}
	for _, s := range rc.Steps {
		target[s.Provider] = true
	}

	source := map[string]*provider.Setting{}
	for _, s := range settings {
		source[s.Provider] = s
	}

	for p := range target {
		if source[p] == nil {
			return []*provider.Setting{}
		}

		selected = append(selected, source[p])
	}

	return selected
}

func canAccessPath(provider string, path string) bool {
	if provider == "bedrock" && !strings.HasPrefix(path, "/api/providers/bedrock") {
		return false
	}

	if provider == "openai" && !strings.HasPrefix(path, "/api/providers/openai") {
		return false
	}

	if provider == "azure" && !strings.HasPrefix(path, "/api/providers/azure/openai") {
		return false
	}

	if provider == "anthropic" && !strings.HasPrefix(path, "/api/providers/anthropic") {
		return false
	}

	if provider == "vllm" && !strings.HasPrefix(path, "/api/providers/vllm") {
		return false
	}

	return true
}

type notFoundError interface {
	Error() string
	NotFound()
}

func anonymize(input string) string {
	if len(input) == 0 {
		return ""
	}

	if len(input) <= 5 && len(input) >= 1 {
		return string(input[0]) + "*****"
	}

	return string(input[0:5]) + "**********************************************"
}

func (a *Authenticator) AuthenticateHttpRequest(req *http.Request, xCustomProviderId string) (*key.ResponseKey, []*provider.Setting, error) {
	var raw string
	var err error
	var settings []*provider.Setting
	if xcustom.IsXCustomRequest(req) {
		providerSetting, er := a.psm.GetSettingViaCache(xCustomProviderId)
		if er != nil {
			return nil, nil, er
		}
		settings = []*provider.Setting{providerSetting}
		raw, err = xcustom.ExtractApiKey(req, providerSetting)
	} else {
		raw, err = getApiKey(req)
	}
	if err != nil {
		return nil, nil, err
	}

	hash := hasher.Hash(raw)

	key, err := a.kc.GetKeyViaCache(hash)
	if key != nil {
		telemetry.Incr(metricname.COUNTER_AUTHENTICATOR_FOUND_KEY_FROM_MEMDB, nil, 1)
	}

	if key == nil {
		key, err = a.kc.GetKeyViaCache(raw)
	}

	if err != nil {
		_, ok := err.(notFoundError)
		if ok {
			return nil, nil, internal_errors.NewAuthError(fmt.Sprintf("key %s is not found", anonymize(raw)))
		}

		return nil, nil, err
	}

	if key == nil {
		return nil, nil, internal_errors.NewAuthError(fmt.Sprintf("key %s is not found", anonymize(raw)))
	}

	if key.Revoked {
		return nil, nil, internal_errors.NewAuthError(fmt.Sprintf("key %s has been revoked", anonymize(raw)))
	}

	if xcustom.IsXCustomRequest(req) {
		pSetting := settings[0]
		authString := strings.Replace(
			pSetting.GetParam(xcustom.XCustomSettingFields.AuthMask),
			"{{apikey}}",
			pSetting.GetParam(xcustom.XCustomSettingFields.ApiKey), -1,
		)
		location := xcustom.GetAuthLocation(pSetting.GetParam(xcustom.XCustomSettingFields.AuthLocation))
		target := pSetting.GetParam(xcustom.XCustomSettingFields.AuthTarget)
		switch location {
		case xcustom.AuthLocations.Query:
			params := req.URL.Query()
			params.Set(target, authString)
			req.URL.RawQuery = params.Encode()
		case xcustom.AuthLocations.Header:
			req.Header.Set(target, authString)
		default:
			return nil, nil, errors.New("invalid xCustomAuth location")
		}
		return key, settings, nil
	}

	if strings.HasPrefix(req.URL.Path, "/api/routes") {
		err = a.canKeyAccessCustomRoute(req.URL.Path, key.KeyId)
		if err != nil {
			return nil, nil, err
		}
	}

	settingIds := key.GetSettingIds()
	allSettings := []*provider.Setting{}
	selected := []*provider.Setting{}
	for _, settingId := range settingIds {
		setting, _ := a.psm.GetSettingViaCache(settingId)
		if setting == nil {
			telemetry.Incr("bricksllm.authenticator.authenticate_http_request.get_setting_error", nil, 1)
			continue
		}

		if canAccessPath(setting.Provider, req.URL.Path) {
			selected = append(selected, setting)
		}

		allSettings = append(allSettings, setting)
	}

	if strings.HasPrefix(req.URL.Path, "/api/routes") {
		selected = a.getProviderSettingsThatCanAccessCustomRoute(req.URL.Path, allSettings)

		if len(selected) == 0 {
			return nil, nil, internal_errors.NewAuthError(fmt.Sprintf("provider settings associated with the key %s are not compatible with the route", anonymize(raw)))
		}
	}

	if len(selected) != 0 {
		used := selected[0]
		if key.RotationEnabled {
			used = selected[rand.Intn(len(selected))]
		}

		if a.decryptor.Enabled() {
			encryptedParam := ""
			if used.Provider == "amazon" {
				encryptedParam = used.Setting["awsSecretAccessKey"]
			} else if len(used.Setting["apikey"]) != 0 {
				encryptedParam = used.Setting["apikey"]
			}

			if len(encryptedParam) != 0 {
				decryptedSecret, err := a.decryptor.Decrypt(encryptedParam, map[string]string{"X-UPDATED-AT": strconv.FormatInt(used.UpdatedAt, 10)})
				if err == nil {
					if used.Provider == "amazon" {
						used.Setting["awsSecretAccessKey"] = decryptedSecret
					} else {
						used.Setting["apikey"] = decryptedSecret
					}
				}
			}
		}

		err := rewriteHttpAuthHeader(req, used)
		if err != nil {
			return nil, nil, err
		}

		return key, selected, nil
	}

	return nil, nil, internal_errors.NewAuthError(fmt.Sprintf("provider setting not found for key %s", raw))
}
