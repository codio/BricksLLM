package validator

import (
	"errors"
	"fmt"
	"time"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/key"
)

type costLimitCache interface {
	GetCounter(keyId string, rateLimitUnit key.TimeUnit) (int64, error)
}

type rateLimitCache interface {
	GetCounter(keyId string, rateLimitUnit key.TimeUnit) (int64, error)
}

type costLimitStorage interface {
	GetCounter(keyId string) (int64, error)
}

type requestsLimitStorage interface {
	GetCounter(keyId string) (int64, error)
}

type Validator struct {
	clc  costLimitCache
	rlc  rateLimitCache
	cls  costLimitStorage
	rqls requestsLimitStorage
}

func NewValidator(
	clc costLimitCache,
	rlc rateLimitCache,
	cls costLimitStorage,
	rqls requestsLimitStorage,
) *Validator {
	return &Validator{
		clc:  clc,
		rlc:  rlc,
		cls:  cls,
		rqls: rqls,
	}
}

func (v *Validator) Validate(k *key.ResponseKey, promptCost float64) error {
	if k == nil {
		return internal_errors.NewValidationError("empty api key")
	}

	if k.Revoked {
		return internal_errors.NewValidationError("api key revoked")
	}

	parsed, _ := time.ParseDuration(k.Ttl)
	if !v.validateTtl(k.CreatedAt, parsed) {
		return internal_errors.NewExpirationError("api key expired", internal_errors.TtlExpiration)
	}

	err := v.validateRequestsLimit(k.KeyId, k.RequestsLimit)
	if err != nil {
		return err
	}

	err = v.validateRateLimitOverTime(k.KeyId, k.RateLimitOverTime, k.RateLimitUnit)
	if err != nil {
		return err
	}

	err = v.validateCostLimitOverTime(k.KeyId, k.CostLimitInUsdOverTime, k.CostLimitInUsdUnit)
	if err != nil {
		return err
	}

	err = v.validateCostLimit(k.KeyId, k.CostLimitInUsd)
	if err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateTtl(createdAt int64, ttl time.Duration) bool {
	ttlInSecs := int64(ttl.Seconds())

	if ttlInSecs == 0 {
		return true
	}

	current := time.Now().Unix()
	return current < createdAt+ttlInSecs
}

func (v *Validator) validateRateLimitOverTime(keyId string, rateLimitOverTime int, rateLimitUnit key.TimeUnit) error {
	if rateLimitOverTime == 0 {
		return nil
	}

	c, err := v.rlc.GetCounter(keyId, rateLimitUnit)
	if err != nil {
		return errors.New("failed to get rate limit counter")
	}

	if c >= int64(rateLimitOverTime) {
		return internal_errors.NewRateLimitError(fmt.Sprintf("key exceeded rate limit %d requests per %s", rateLimitOverTime, rateLimitUnit))
	}

	return nil
}

func (v *Validator) validateCostLimitOverTime(keyId string, costLimitOverTime float64, costLimitUnit key.TimeUnit) error {
	if costLimitOverTime == 0 {
		return nil
	}

	cachedCost, err := v.clc.GetCounter(keyId, costLimitUnit)
	if err != nil {
		return errors.New("failed to get cached token cost")
	}

	if cachedCost >= convertDollarToMicroDollars(costLimitOverTime) {
		return internal_errors.NewCostLimitError(fmt.Sprintf("cost limit: %f has been reached for the current time period: %s", costLimitOverTime, costLimitUnit))
	}

	return nil
}

func convertDollarToMicroDollars(dollar float64) int64 {
	return int64(dollar * 1000000)
}

func (v *Validator) validateCostLimit(keyId string, costLimit float64) error {
	if costLimit == 0 {
		return nil
	}

	existingTotalCost, err := v.cls.GetCounter(keyId)
	if err != nil {
		return errors.New("failed to get total token cost")
	}

	if existingTotalCost >= convertDollarToMicroDollars(costLimit) {
		return internal_errors.NewExpirationError(fmt.Sprintf("total cost limit: %f has been reached", costLimit), internal_errors.CostLimitExpiration)
	}

	return nil
}

func (v *Validator) validateRequestsLimit(keyId string, requestsLimit int) error {
	if requestsLimit == 0 {
		return nil
	}
	existingTotalRequests, err := v.rqls.GetCounter(keyId)
	if err != nil {
		return errors.New("failed to get total requests")
	}
	if existingTotalRequests >= int64(requestsLimit) {
		return internal_errors.NewExpirationError(fmt.Sprintf("total requests limit: %d, has been reached", requestsLimit), internal_errors.RequestsLimitExpiration)
	}
	return nil
}
