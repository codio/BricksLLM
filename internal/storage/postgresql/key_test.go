package postgresql

import (
	"reflect"
	"testing"

	"github.com/bricks-cloud/bricksllm/internal/key"
)

func TestMarshalExtendedBudgetLimitNil(t *testing.T) {
	data, err := marshalExtendedBudgetLimit(nil)
	if err != nil {
		t.Fatalf("marshalExtendedBudgetLimit(nil) returned error: %v", err)
	}

	if data != nil {
		t.Fatalf("marshalExtendedBudgetLimit(nil) = %q, want nil", data)
	}
}

func TestExtendedBudgetLimitJSONRoundTrip(t *testing.T) {
	limit := &key.ExtendedBudgetLimit{
		Items: []key.ExtendedBudgetLimitItem{
			{LimitInUsdOverTime: 1.5, Unit: key.HourTimeUnit},
			{LimitInUsdOverTime: 2.5, Unit: key.DayTimeUnit},
		},
	}

	data, err := marshalExtendedBudgetLimit(limit)
	if err != nil {
		t.Fatalf("marshalExtendedBudgetLimit returned error: %v", err)
	}

	roundTripped, err := unmarshalExtendedBudgetLimit(data)
	if err != nil {
		t.Fatalf("unmarshalExtendedBudgetLimit returned error: %v", err)
	}

	if !reflect.DeepEqual(roundTripped, limit) {
		t.Fatalf("round trip mismatch: got %#v, want %#v", roundTripped, limit)
	}
}

func TestUnmarshalExtendedBudgetLimitNilHandling(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "sql null", data: nil},
		{name: "json null", data: []byte("null")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, err := unmarshalExtendedBudgetLimit(tt.data)
			if err != nil {
				t.Fatalf("unmarshalExtendedBudgetLimit returned error: %v", err)
			}

			if limit != nil {
				t.Fatalf("unmarshalExtendedBudgetLimit(%q) = %#v, want nil", tt.data, limit)
			}
		})
	}
}

func TestUnmarshalExtendedBudgetLimitInvalidJSON(t *testing.T) {
	if _, err := unmarshalExtendedBudgetLimit([]byte("{")); err == nil {
		t.Fatal("unmarshalExtendedBudgetLimit should fail for invalid JSON")
	}
}
