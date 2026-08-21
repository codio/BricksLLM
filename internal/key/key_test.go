package key

import "testing"

func TestValidateExtendedBudgetLimit(t *testing.T) {
	tests := []struct {
		name    string
		ebl     *ExtendedBudgetLimit
		wantErr string
	}{
		{
			name:    "nil limit is valid",
			ebl:     nil,
			wantErr: "",
		},
		{
			name: "duplicate units are rejected",
			ebl: &ExtendedBudgetLimit{
				Items: []ExtendedBudgetLimitItem{
					{LimitInUsdOverTime: 1, Unit: HourTimeUnit},
					{LimitInUsdOverTime: 2, Unit: HourTimeUnit},
				},
			},
			wantErr: "extendedBudgetLimit.items[1].unit is duplicated",
		},
		{
			name: "unique units are valid",
			ebl: &ExtendedBudgetLimit{
				Items: []ExtendedBudgetLimitItem{
					{LimitInUsdOverTime: 1, Unit: HourTimeUnit},
					{LimitInUsdOverTime: 2, Unit: MinuteTimeUnit},
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExtendedBudgetLimit(tt.ebl)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
