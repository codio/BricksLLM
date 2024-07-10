package event

type KeyDataPoint struct {
	KeyId     string  `json:"keyId"`
	CostInUsd float64 `json:"costInUsd"`
}

type KeyReportingResponse struct {
	DataPoints []*KeyDataPoint `json:"dataPoints"`
}

type KeyReportingRequest struct {
	Tags    []string `json:"tags"`
	Order   string   `json:"order"`
	KeyIds  []string `json:"keyIds"`
	Start   int64    `json:"start"`
	End     int64    `json:"end"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	Name    string   `json:"name"`
	Revoked *bool    `json:"revoked"`
}

type KeyRingReportingRequest struct {
	Tags    []string `json:"tags"`
	Order   string   `json:"order"`
	Start   int64    `json:"start"`
	End     int64    `json:"end"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	Revoked *bool    `json:"revoked"`
}

type KeyRingDataPoint struct {
	KeyRing   string  `json:"keyRing"`
	CostInUsd float64 `json:"costInUsd"`
}

type KeyRingReportingResponse struct {
	DataPoints []*KeyRingDataPoint `json:"dataPoints"`
}

type SpentKeyReportingRequest struct {
	Tags   []string `json:"tags"`
	Order  string   `json:"order"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type SpentKeyDataPoint struct {
	KeyRing string
	KeyId   string
}

type SpentKeyReportingResponse struct {
	KeyRings []string `json:"keyRings"`
}

type UsageReportingRequest struct {
	Tags []string `json:"tags"`
}

type UsageData struct {
	LastDayUsage   float64 `json:"lastDayUsage"`
	LastWeekUsage  float64 `json:"lastWeekUsage"`
	LastMonthUsage float64 `json:"lastMonthUsage"`
	TotalUsage     float64 `json:"totalUsage"`
}

type UsageReportingResponse struct {
	UsageData *UsageData `json:"usageData"`
}
