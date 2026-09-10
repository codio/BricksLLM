package manager

import (
	"strings"
	"time"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/event"
	"github.com/bricks-cloud/bricksllm/internal/key"
)

type costStorage interface {
	GetCounter(keyId string) (int64, error)
}

type keyStorage interface {
	GetKey(keyId string) (*key.ResponseKey, error)
	GetSpentKeys(tags []string, order string, limit, offset int, validator func(*key.ResponseKey) bool) ([]event.SpentKey, error)
}

type keyValidator interface {
	Validate(k *key.ResponseKey, promptCost float64) error
}

type StatisticsCache interface {
	Get(key string) (*event.StatisticsData, error)
	Set(key string, val *event.StatisticsData, ttl time.Duration) error
	SetInProgress(key string) error
	DeleteInProgress(key string) error
	IsInProgress(key string) bool
}

type eventStorage interface {
	GetEvents(userId, customId string, keyIds []string, start, end int64) ([]*event.Event, error)
	GetEventsV2(req *event.EventRequest) (*event.EventResponse, error)
	GetEventDataPoints(start, end, increment int64, tags, keyIds, customIds, userIds []string, filters []string) ([]*event.DataPoint, error)
	GetLatencyPercentiles(start, end int64, tags, keyIds []string) ([]float64, error)
	GetAggregatedEventByDayDataPoints(start, end int64, keyIds []string) ([]*event.DataPointV2, error)
	GetUserIds(keyId string) ([]string, error)
	GetCustomIds(keyId string) ([]string, error)
	GetTopKeyDataPoints(start, end int64, tags, keyIds []string, order string, limit, offset int, name string, revoked *bool) ([]*event.KeyDataPoint, error)

	GetTopKeyRingDataPoints(start, end int64, tags []string, order string, limit, offset int, revoked *bool, topBy string) ([]*event.KeyRingDataPoint, error)
	GetUsageData(tags []string) (*event.UsageData, error)
	GetStatisticsData(level event.StatisticLevel, id *string) (*event.StatisticsData, error)
}

type ReportingManager struct {
	es eventStorage
	cs costStorage
	ks keyStorage
	kv keyValidator
	sc StatisticsCache
}

func NewReportingManager(cs costStorage, ks keyStorage, es eventStorage, kv keyValidator, sc StatisticsCache) *ReportingManager {
	return &ReportingManager{
		cs: cs,
		ks: ks,
		es: es,
		kv: kv,
		sc: sc,
	}
}

func (rm *ReportingManager) GetEventReporting(e *event.ReportingRequest) (*event.ReportingResponse, error) {
	dataPoints, err := rm.es.GetEventDataPoints(e.Start, e.End, e.Increment, e.Tags, e.KeyIds, e.CustomIds, e.UserIds, e.Filters)
	if err != nil {
		return nil, err
	}

	percentiles, err := rm.es.GetLatencyPercentiles(e.Start, e.End, e.Tags, e.KeyIds)
	if err != nil {
		return nil, err
	}

	if len(percentiles) == 0 {
		return nil, internal_errors.NewNotFoundError("latency percentiles are not found")
	}

	return &event.ReportingResponse{
		DataPoints:        dataPoints,
		LatencyInMsMedian: percentiles[0],
		LatencyInMs99th:   percentiles[1],
	}, nil
}

func (rm *ReportingManager) GetAggregatedEventByDayReporting(e *event.ReportingRequest) (*event.ReportingResponseV2, error) {
	dataPoints, err := rm.es.GetAggregatedEventByDayDataPoints(e.Start, e.End, e.KeyIds)
	if err != nil {
		return nil, err
	}

	return &event.ReportingResponseV2{
		DataPoints: dataPoints,
	}, nil
}

func (rm *ReportingManager) GetTopKeyReporting(r *event.KeyReportingRequest) (*event.KeyReportingResponse, error) {
	if r == nil {
		return nil, internal_errors.NewValidationError("key reporting requst cannot be nil")
	}

	for _, tag := range r.Tags {
		if len(tag) == 0 {
			return nil, internal_errors.NewValidationError("key reporting requst tag cannot be empty")
		}
	}

	for _, kid := range r.KeyIds {
		if len(kid) == 0 {
			return nil, internal_errors.NewValidationError("key reporting requst key id cannot be empty")
		}
	}

	if len(r.Order) != 0 && strings.ToUpper(r.Order) != "DESC" && strings.ToUpper(r.Order) != "ASC" {
		return nil, internal_errors.NewValidationError("key reporting request order can only be desc or asc")
	}

	dataPoints, err := rm.es.GetTopKeyDataPoints(r.Start, r.End, r.Tags, r.KeyIds, r.Order, r.Limit, r.Offset, r.Name, r.Revoked)
	if err != nil {
		return nil, err
	}

	return &event.KeyReportingResponse{
		DataPoints: dataPoints,
	}, nil
}

func (rm *ReportingManager) GetTopKeyRingReporting(r *event.KeyRingReportingRequest) (*event.KeyRingReportingResponse, error) {
	if r == nil {
		return nil, internal_errors.NewValidationError("key reporting requst cannot be nil")
	}

	for _, tag := range r.Tags {
		if len(tag) == 0 {
			return nil, internal_errors.NewValidationError("key reporting requst tag cannot be empty")
		}
	}

	if len(r.Order) != 0 && strings.ToUpper(r.Order) != "DESC" && strings.ToUpper(r.Order) != "ASC" {
		return nil, internal_errors.NewValidationError("key reporting request order can only be desc or asc")
	}

	dataPoints, err := rm.es.GetTopKeyRingDataPoints(r.Start, r.End, r.Tags, r.Order, r.Limit, r.Offset, r.Revoked, r.TopBy)
	if err != nil {
		return nil, err
	}

	return &event.KeyRingReportingResponse{
		DataPoints: dataPoints,
	}, nil
}

func (rm *ReportingManager) GetSpentKeyReporting(r *event.SpentKeyReportingRequest) (*event.SpentKeyReportingResponse, error) {
	if r == nil {
		return nil, internal_errors.NewValidationError("key reporting requst cannot be nil")
	}
	for _, tag := range r.Tags {
		if len(tag) == 0 {
			return nil, internal_errors.NewValidationError("key reporting requst tag cannot be empty")
		}
	}
	if len(r.Order) != 0 && strings.ToUpper(r.Order) != "DESC" && strings.ToUpper(r.Order) != "ASC" {
		return nil, internal_errors.NewValidationError("key reporting request order can only be desc or asc")
	}

	validator := func(k *key.ResponseKey) bool {
		if e := rm.kv.Validate(k, 0); e != nil {
			return false
		}
		return true
	}

	spentKeys, err := rm.ks.GetSpentKeys(r.Tags, r.Order, r.Limit, r.Offset, validator)
	if err != nil {
		return nil, err
	}
	return &event.SpentKeyReportingResponse{
		Keys: spentKeys,
	}, nil
}

func (rm *ReportingManager) GetUsageReporting(r *event.UsageReportingRequest) (*event.UsageReportingResponse, error) {
	if r == nil {
		return nil, internal_errors.NewValidationError("key reporting request cannot be nil")
	}
	for _, tag := range r.Tags {
		if len(tag) == 0 {
			return nil, internal_errors.NewValidationError("key reporting request tag cannot be empty")
		}
	}
	usage, err := rm.es.GetUsageData(r.Tags)
	if err != nil {
		return nil, err
	}
	return &event.UsageReportingResponse{
		UsageData: usage,
	}, nil
}

func (rm *ReportingManager) GetStatistic(r *event.StatisticsRequest) (*event.StatisticsResponse, error) {
	if r == nil {
		return nil, internal_errors.NewValidationError("statistics request cannot be nil")
	}

	if err := r.Validate(); err != nil {
		return nil, err
	}

	cacheKey := r.GetCacheKey()
	if data, err := rm.sc.Get(cacheKey); err == nil {
		return &event.StatisticsResponse{
			StatisticsData: data,
		}, nil
	}

	go rm.backgroundCollectStatisticsData(cacheKey, r.GetLevel(), r.Id)

	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, internal_errors.NewNotFoundError("statistics data is not ready yet, please try again later")
		case <-ticker.C:
			if data, err := rm.sc.Get(cacheKey); err == nil {
				return &event.StatisticsResponse{
					StatisticsData: data,
				}, nil
			}
		}
	}
}

func (rm *ReportingManager) backgroundCollectStatisticsData(cacheKey string, level event.StatisticLevel, id *string) {
	if rm.sc.IsInProgress(cacheKey) {
		return
	}
	err := rm.sc.SetInProgress(cacheKey)
	if err != nil {
		return
	}
	defer rm.sc.DeleteInProgress(cacheKey)

	statisticsData, err := rm.es.GetStatisticsData(level, id)

	if err != nil {
		return
	}
	_ = rm.sc.Set(cacheKey, statisticsData, time.Hour*24)
}

func (rm *ReportingManager) GetCustomIds(keyId string) ([]string, error) {
	return rm.es.GetCustomIds(keyId)
}

func (rm *ReportingManager) GetUserIds(keyId string) ([]string, error) {
	return rm.es.GetUserIds(keyId)
}

func (rm *ReportingManager) GetKeyReporting(keyId string) (*key.KeyReporting, error) {
	k, err := rm.ks.GetKey(keyId)
	if err != nil {
		return nil, err
	}

	if k == nil {
		return nil, internal_errors.NewNotFoundError("api key is not found")
	}

	micros, err := rm.cs.GetCounter(keyId)
	if err != nil {
		return nil, err
	}

	return &key.KeyReporting{
		Id:                 keyId,
		CostInMicroDollars: micros,
	}, err
}

func (rm *ReportingManager) GetEvents(userId, customId string, keyIds []string, start, end int64) ([]*event.Event, error) {
	events, err := rm.es.GetEvents(userId, customId, keyIds, start, end)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (rm *ReportingManager) GetEventsV2(req *event.EventRequest) (*event.EventResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	resp, err := rm.es.GetEventsV2(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
