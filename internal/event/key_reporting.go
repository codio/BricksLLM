package event

import (
	internalErrors "github.com/bricks-cloud/bricksllm/internal/errors"
)

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
	TopBy   string   `json:"topBy"`
}

type KeyRingDataPoint struct {
	KeyRing   string  `json:"keyRing"`
	CostInUsd float64 `json:"costInUsd"`
	Requests  int     `json:"requests"`
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

type SpentKey struct {
	KeyRing     string `json:"keyRing"`
	LinkedKeyId string `json:"linkedKeyId"`
}

type SpentKeyReportingResponse struct {
	Keys []SpentKey `json:"keys"`
}

type UsageReportingRequest struct {
	Tags []string `json:"tags"`
}

type UsageData struct {
	LastDayUsage           float64 `json:"lastDayUsage"`
	LastWeekUsage          float64 `json:"lastWeekUsage"`
	LastMonthUsage         float64 `json:"lastMonthUsage"`
	TotalUsage             float64 `json:"totalUsage"`
	LastDayUsageRequests   int     `json:"lastDayUsageRequests"`
	LastWeekUsageRequests  int     `json:"lastWeekUsageRequests"`
	LastMonthUsageRequests int     `json:"lastMonthUsageRequests"`
	TotalUsageRequests     int     `json:"totalUsageRequests"`
}

type UsageReportingResponse struct {
	UsageData *UsageData `json:"usageData"`
}

type StatisticsRequest struct {
	Level string `json:"level"` // all | org | course
	Id *string `json:"id"`
}

type StatisticLevel string

var StisticLevels = struct {
	Unknown StatisticLevel
	All    StatisticLevel
	Org    StatisticLevel
	Course StatisticLevel

}{
	Unknown: "unknown",
	All:    "all",
	Org:    "org",
	Course: "course",
}

func StatisticLevelFromStr(s string) StatisticLevel {
	switch s {
	case "all":
		return StisticLevels.All
	case "org":
		return StisticLevels.Org
	case "course":
		return StisticLevels.Course
	default:
		return StisticLevels.Unknown
	}
}

func (r *StatisticsRequest) Validate() error {
	if r.Level != "all" && r.Level != "org" && r.Level != "course" {
		return internalErrors.NewValidationError("level must be one of 'all', 'org', or 'course'")
	}
	if (r.Level == "org" || r.Level == "course") && r.Id == nil {
		return internalErrors.NewValidationError("id must be provided when level is 'org' or 'course'")
	}
	return nil
}

func (r *StatisticsRequest) GetLevel() StatisticLevel {
	return StatisticLevelFromStr(r.Level)
}

type StatisticsResponse struct {
	StatisticsData *StatisticsData `json:"statisticsData"`
}

type StatisticsData struct {
	AllStatisticsData *AllStatisticsData `json:"allStatisticsData,omitempty"`
	OrgStatisticsData *OrgStatisticsData `json:"orgStatisticsData,omitempty"`
	CourseStatisticsData *CourseStatisticsData `json:"courseStatisticsData,omitempty"`
}

type Cost struct {
	OneMonth float64 `json:"1month"`
	FiveMonth float64 `json:"5month"`
}

type CostPack struct {
	CodioProvided Cost `json:"codioProvided"`
	CodioSpecial  Cost `json:"codioSpecial"`
}

type BellCurveDataPoint struct {
	CostBucketUsd string `json:"costBucketUsd"`
	UserCount     int    `json:"userCount"`
	SerialNo      int    `json:"serialNo"`
}

type FinancialKpis struct {
	MaxUserSpend    float64 `json:"maxUserSpend"`
	MedianUserSpend float64 `json:"medianUserSpend"`
	AvgUserSpend    float64 `json:"avgUserSpend"`
}

type AllStatisticsData struct {
	Total CostPack `json:"total"`
	Orgs  []ShortOrgStatisticsData `json:"orgs"`
}

type ShortOrgStatisticsData struct {
	Id    string   `json:"id"`
	Costs CostPack `json:"costs"`
}

type OrgStatisticsData struct {
	Id      string   `json:"id"`
	Costs   CostPack `json:"costs"`
	Courses []CourseStatisticsData `json:"courses"`
	BellCurveSpecial []BellCurveDataPoint `json:"bellCurveSpecial"`
	FinancialKpisSpecial FinancialKpis `json:"financialKpisSpecial"`
}

type CourseStatisticsData struct {
	Id      string   `json:"id"`
	Costs   CostPack `json:"costs"`
	BellCurveCodioProvided []BellCurveDataPoint `json:"bellCurveCodioProvided"`
	FinancialKpisCodioProvided FinancialKpis `json:"financialKpisCodioProvided"`
}
