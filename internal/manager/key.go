package manager

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/hasher"
	"github.com/bricks-cloud/bricksllm/internal/key"
	"github.com/bricks-cloud/bricksllm/internal/policy"
	"github.com/bricks-cloud/bricksllm/internal/provider"
	secondarykey "github.com/bricks-cloud/bricksllm/internal/secondary-key"
	"github.com/bricks-cloud/bricksllm/internal/telemetry"
	"github.com/bricks-cloud/bricksllm/internal/util"
)

type Storage interface {
	GetKeys(tags, keyIds []string, provider string) ([]*key.ResponseKey, error)
	GetKeysV2(tags, keyIds []string, revoked *bool, limit, offset int, name, order string, returnCount bool) (*key.GetKeysResponse, error)
	UpdateKey(id string, key *key.UpdateKey) (*key.ResponseKey, error)
	CreateKey(key *key.RequestKey) (*key.ResponseKey, error)
	DeleteKey(id string) error
	GetProviderSetting(id string, withSecret bool) (*provider.Setting, error)
	GetPolicyById(id string) (*policy.Policy, error)
	GetProviderSettings(withSecret bool, ids []string) ([]*provider.Setting, error)
	GetKey(keyId string) (*key.ResponseKey, error)
	GetKeyByHash(hash string) (*key.ResponseKey, error)
	GetKeyHashBySecondary(sHash string) (string, error)
	CreateSecondaryKey(secondaryHash string) error
	UpdateSecondaryKey(secondaryHash, keyHash string) error
}

type costLimitCache interface {
	Delete(keyId string) error
}

type rateLimitCache interface {
	Delete(keyId string) error
}

type accessCache interface {
	Delete(keyId string) error
}

type keyCache interface {
	Set(keyId string, value interface{}, ttl time.Duration) error
	Delete(keyId string) error
	Get(keyId string) (*key.ResponseKey, error)
}

type secondaryKeyCache interface {
	Set(sHash string, value string, ttl time.Duration) error
	Delete(sHash string) error
	Get(sHash string) (string, error)
}

type requestsLimitStorage interface {
	DeleteCounter(keyId string) error
}

type Manager struct {
	s           Storage
	clc         costLimitCache
	rlc         rateLimitCache
	ac          accessCache
	kc          keyCache
	secondaryKC secondaryKeyCache
	rqls        requestsLimitStorage
}

func NewManager(s Storage, clc costLimitCache, rlc rateLimitCache, ac accessCache, kc keyCache, secondaryKC secondaryKeyCache, rqls requestsLimitStorage) *Manager {
	return &Manager{
		s:           s,
		clc:         clc,
		rlc:         rlc,
		ac:          ac,
		kc:          kc,
		secondaryKC: secondaryKC,
		rqls:        rqls,
	}
}

func (m *Manager) GetKeysV2(tags, keyIds []string, revoked *bool, limit, offset int, name, order string, returnCount bool) (*key.GetKeysResponse, error) {
	if len(order) != 0 && strings.ToUpper(order) != "DESC" && strings.ToUpper(order) != "ASC" {
		return nil, internal_errors.NewValidationError("get keys request order can only be desc or asc")
	}

	return m.s.GetKeysV2(tags, keyIds, revoked, limit, offset, name, order, returnCount)
}

func (m *Manager) GetKeys(tags, keyIds []string, provider string) ([]*key.ResponseKey, error) {
	return m.s.GetKeys(tags, keyIds, provider)
}

func (m *Manager) CreateKey(rk *key.RequestKey) (*key.ResponseKey, error) {
	rk.CreatedAt = time.Now().Unix()
	rk.UpdatedAt = time.Now().Unix()
	rk.KeyId = util.NewUuid()

	if err := rk.Validate(); err != nil {
		return nil, err
	}

	if !rk.IsKeyNotHashed {
		rk.Key = hasher.Hash(rk.Key)
	}

	if len(rk.SettingId) != 0 {
		if _, err := m.s.GetProviderSetting(rk.SettingId, false); err != nil {
			return nil, err
		}
	}

	if len(rk.SettingIds) != 0 {
		existing, err := m.s.GetProviderSettings(false, rk.SettingIds)
		if err != nil {
			return nil, err
		}

		if len(existing) == 0 {
			return nil, errors.New("provider settings not found")
		}
	}

	if len(rk.PolicyId) != 0 {
		_, err := m.s.GetPolicyById(rk.PolicyId)
		if err != nil {
			return nil, err
		}
	}

	return m.s.CreateKey(rk)
}

func (m *Manager) UpdateKey(id string, uk *key.UpdateKey) (*key.ResponseKey, error) {
	uk.UpdatedAt = time.Now().Unix()

	if err := uk.Validate(); err != nil {
		return nil, err
	}

	existing, err := m.s.GetKey(id)
	if err != nil {
		return nil, err
	}

	current := existing.Key

	if uk.IsKeyNotHashed != nil && !*uk.IsKeyNotHashed {
		uk.Key = hasher.Hash(existing.Key)
	}

	if uk.IsKeyNotHashed == nil || (uk.IsKeyNotHashed != nil && *uk.IsKeyNotHashed) {
		uk.Key = ""
	}

	if len(uk.SettingId) != 0 {
		if _, err := m.s.GetProviderSetting(uk.SettingId, false); err != nil {
			return nil, err
		}
	}

	if len(uk.SettingIds) != 0 {
		existing, err := m.s.GetProviderSettings(false, uk.SettingIds)
		if err != nil {
			return nil, err
		}

		if len(existing) == 0 {
			return nil, errors.New("provider settings not found")
		}
	}

	if uk.CostLimitInUsdUnit != nil {
		err := m.clc.Delete(id)
		if err != nil {
			return nil, err
		}
	}

	if uk.RateLimitUnit != nil {
		err := m.rlc.Delete(id)
		if err != nil {
			return nil, err
		}
	}

	if uk.CostLimitInUsdUnit != nil || uk.RateLimitUnit != nil {
		err := m.ac.Delete(id)
		if err != nil {
			return nil, err
		}
	}
	if uk.RequestsLimit != nil {
		err := m.rqls.DeleteCounter(id)
		if err != nil {
			return nil, err
		}
	}

	if uk.PolicyId != nil {
		if len(*uk.PolicyId) != 0 {
			_, err := m.s.GetPolicyById(*uk.PolicyId)
			if err != nil {
				return nil, err
			}
		}
	}

	updated, err := m.s.UpdateKey(id, uk)
	if err != nil {
		return nil, err
	}

	err = m.kc.Delete(current)
	if err != nil {
		telemetry.Incr("bricksllm.manager.update_key.delete_cache_error", nil, 1)
	}

	return updated, nil
}

func (m *Manager) GetKeyViaCache(raw string) (*key.ResponseKey, error) {
	k, _ := m.kc.Get(raw)

	if k == nil {
		telemetry.Incr("bricksllm.manager.get_key_via_cache.cache_miss", nil, 1)

		stored, err := m.s.GetKeyByHash(raw)
		if err != nil {
			return nil, err
		}

		bs, err := json.Marshal(stored)
		if err != nil {
			return stored, nil
		}

		err = m.kc.Set(raw, bs, 24*time.Hour)
		if err != nil {
			telemetry.Incr("bricksllm.manager.get_key_via_cache.set_error", nil, 1)
		}

		k = stored
	}

	if k != nil {
		telemetry.Incr("bricksllm.manager.get_key_via_cache.cache_hit", nil, 1)
	}

	return k, nil
}

func (m *Manager) GetKeyHashBySecondary(sHash string) (string, error) {
	h, _ := m.secondaryKC.Get(sHash)
	if h == "" {
		telemetry.Incr("bricksllm.manager.get_key_hash_by_secondary.cache_miss", nil, 1)
		stored, err := m.s.GetKeyHashBySecondary(sHash)
		if err != nil {
			return "", err
		}
		if stored == "" {
			return "", errors.New("key hash not found")
		}
		err = m.secondaryKC.Set(sHash, stored, 24*time.Hour)
		if err != nil {
			telemetry.Incr("bricksllm.manager.get_key_hash_by_secondary.set_error", nil, 1)
		}
		h = stored
		return h, nil
	}
	telemetry.Incr("bricksllm.manager.get_key_hash_by_secondary.cache_hit", nil, 1)
	return h, nil
}

func (m *Manager) CreateSecondaryKey(keyCreate secondarykey.SecondaryKeyCreate) error {
	if keyCreate.Key == "" {
		return errors.New("key is required for creating secondary key")
	}
	return m.s.CreateSecondaryKey(hasher.Hash(keyCreate.Key))
}

func (m *Manager) UpdateSecondaryKey(keyUpdate secondarykey.SecondaryKeyUpdate) error {
	if keyUpdate.Key == "" {
		return errors.New("key is required for updating secondary key")
	}
	if keyUpdate.LinkedKeyId == "" {
		return errors.New("linkedKeyId is required for updating secondary key")
	}
	rKey, err := m.s.GetKey(keyUpdate.LinkedKeyId)
	if err != nil {
		return err
	}
	if rKey == nil {
		return errors.New("linked key not found for updating secondary key")
	}
	err = m.s.UpdateSecondaryKey(hasher.Hash(keyUpdate.Key), rKey.Key)
	if err != nil {
		return err
	}
	err = m.secondaryKC.Set(hasher.Hash(keyUpdate.Key), rKey.Key, 24*time.Hour)
	if err != nil {
		telemetry.Incr("bricksllm.manager.update_secondary_key.set_cache_error", nil, 1)
	}
	return nil
}

func (m *Manager) DeleteKey(id string) error {
	return m.s.DeleteKey(id)
}
