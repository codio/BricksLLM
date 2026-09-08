package validator

import (
	"testing"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/key"
)

type stubCostLimitCache struct {
	counters map[string]int64
}

func (s *stubCostLimitCache) GetCounter(keyId string, rateLimitUnit key.TimeUnit) (int64, error) {
	return s.counters[keyId], nil
}

func TestValidateExtendedCostLimitOverTimePreservesBreachedUnit(t *testing.T) {
	v := NewValidator(
		&stubCostLimitCache{counters: map[string]int64{
			"test-key-w": convertDollarToMicroDollars(1),
		}},
		nil,
		nil,
		nil,
	)

	err := v.validateExtendedCostLimitOverTime("test-key", &key.ExtendedBudgetLimit{
		Items: []key.ExtendedBudgetLimitItem{{
			LimitInUsdOverTime: 1,
			Unit:               key.WeekTimeUnit,
		}},
	})
	if err == nil {
		t.Fatal("expected cost limit error, got nil")
	}

	cle, ok := err.(*internal_errors.CostLimitError)
	if !ok {
		t.Fatalf("expected CostLimitError, got %T", err)
	}

	if cle.TimeUnit() != string(key.WeekTimeUnit) {
		t.Fatalf("expected time unit %q, got %q", key.WeekTimeUnit, cle.TimeUnit())
	}
}
