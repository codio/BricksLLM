package message

import (
	"testing"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/key"
)

type stubValidator struct {
	err error
}

func (s *stubValidator) Validate(k *key.ResponseKey, promptCost float64) error {
	return s.err
}

type stubAccessCache struct {
	key      string
	timeUnit key.TimeUnit
}

func (s *stubAccessCache) Set(key string, timeUnit key.TimeUnit) error {
	s.key = key
	s.timeUnit = timeUnit
	return nil
}

func TestHandleValidationResultUsesBreachedCostLimitUnitWhenProvided(t *testing.T) {
	ac := &stubAccessCache{}
	h := &Handler{
		v:  &stubValidator{err: internal_errors.NewCostLimitError("extended cost limit reached", string(key.WeekTimeUnit))},
		ac: ac,
	}

	err := h.handleValidationResult(&key.ResponseKey{
		KeyId:              "test-key",
		CostLimitInUsdUnit: key.DayTimeUnit,
	}, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if ac.key != "test-key" {
		t.Fatalf("expected cache key %q, got %q", "test-key", ac.key)
	}

	if ac.timeUnit != key.WeekTimeUnit {
		t.Fatalf("expected time unit %q, got %q", key.WeekTimeUnit, ac.timeUnit)
	}
}

func TestHandleValidationResultFallsBackToKeyCostLimitUnit(t *testing.T) {
	ac := &stubAccessCache{}
	h := &Handler{
		v:  &stubValidator{err: internal_errors.NewCostLimitError("standard cost limit reached")},
		ac: ac,
	}

	err := h.handleValidationResult(&key.ResponseKey{
		KeyId:              "test-key",
		CostLimitInUsdUnit: key.DayTimeUnit,
	}, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if ac.timeUnit != key.DayTimeUnit {
		t.Fatalf("expected time unit %q, got %q", key.DayTimeUnit, ac.timeUnit)
	}
}
