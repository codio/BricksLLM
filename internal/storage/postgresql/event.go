package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	internal_errors "github.com/bricks-cloud/bricksllm/internal/errors"
	"github.com/bricks-cloud/bricksllm/internal/event"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

var allowedTopBy = []string{"total_cost_in_usd", "total_requests"}

func (s *Store) CreateEventsByDayTable() error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS event_agg_by_day (
		id SERIAL PRIMARY KEY,
		time_stamp BIGINT NOT NULL,
		num_of_requests BIGINT NOT NULL,
		cost_in_usd FLOAT8 NOT NULL,
		latency_in_ms BIGINT NOT NULL,
		prompt_token_count BIGINT NOT NULL,
		success_count BIGINT NOT NULL,
		completion_token_count BIGINT NOT NULL,
		key_id VARCHAR(255)
	)`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, createTableQuery)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) AlterEventsTable() error {
	alterTableQuery := `
		ALTER TABLE events ADD COLUMN IF NOT EXISTS path VARCHAR(255), ADD COLUMN IF NOT EXISTS method VARCHAR(255), ADD COLUMN IF NOT EXISTS custom_id VARCHAR(255), ADD COLUMN IF NOT EXISTS request JSONB, ADD COLUMN IF NOT EXISTS response JSONB, ADD COLUMN IF NOT EXISTS user_id VARCHAR(255) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS action VARCHAR(255) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS policy_id VARCHAR(255) NOT NULL DEFAULT '',  ADD COLUMN IF NOT EXISTS route_id VARCHAR(255) NOT NULL DEFAULT '',  ADD COLUMN IF NOT EXISTS correlation_id VARCHAR(255) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS metadata JSONB;
	`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, alterTableQuery)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CreateUniqueIndexForEventsTable() error {
	createIndexQuery := `
	CREATE UNIQUE index IF NOT EXISTS idx_key_id_and_time_stamp on event_agg_by_day (time_stamp, key_id);`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, createIndexQuery)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CreateTimeStampIndexForEventsTable() error {
	createIndexQuery := `
	CREATE index IF NOT EXISTS idx_time_stamp on event_agg_by_day (time_stamp);`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, createIndexQuery)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CreateKeyIdIndexForEventsTable() error {
	createIndexQuery := `
	CREATE index IF NOT EXISTS idx_key_id on event_agg_by_day (key_id);`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, createIndexQuery)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) PrepareEventsIndexes(logger *zap.Logger) error {
	queries := []string{
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_tags ON events USING GIN(tags);`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_created_at_brin ON events USING BRIN(created_at);`,
		`CREATE EXTENSION IF NOT EXISTS btree_gin;`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_tags_created_at_gin ON events USING GIN (tags, created_at);`,
	}

	indexTimeout := 15 * time.Minute
	ctxTimeout, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()

	for _, query := range queries {
		start := time.Now()
		_, err := s.db.ExecContext(ctxTimeout, query)
		if err != nil {
			logger.Sugar().Errorf("error preparing events indexes: %v, with query: %s", err, query)
		}
		execT := time.Since(start).Milliseconds()
		logger.Sugar().Infof("Exec query: %s. time: %d ms", query, execT)
	}

	return nil
}

func (s *Store) CreateEventsTable() error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS events (
		event_id VARCHAR(255) PRIMARY KEY,
		created_at BIGINT NOT NULL,
		tags VARCHAR(255)[],
		key_id VARCHAR(255),
		cost_in_usd FLOAT8,
		provider VARCHAR(255),
		model VARCHAR(255),
		status_code INT,
		prompt_token_count INT,
		completion_token_count INT,
		latency_in_ms INT
	)`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, createTableQuery)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) GetEvents(userId string, customId string, keyIds []string, start int64, end int64) ([]*event.Event, error) {
	if len(customId) == 0 && len(keyIds) == 0 && len(userId) == 0 {
		return nil, errors.New("none of customId, keyIds and userId is specified")
	}

	if len(keyIds) != 0 && (start == 0 || end == 0) {
		return nil, errors.New("keyIds are provided but either start or end is not specified")
	}

	query := `
		SELECT * FROM events WHERE
	`

	if len(customId) != 0 {
		query += fmt.Sprintf(" custom_id = '%s'", customId)
	}

	if len(customId) > 0 && len(userId) > 0 {
		query += " AND"
	}

	if len(userId) != 0 {
		query += fmt.Sprintf(" user_id = '%s'", userId)
	}

	if (len(customId) > 0 || len(userId) > 0) && len(keyIds) > 0 {
		query += " AND"
	}

	if len(keyIds) != 0 {
		query += fmt.Sprintf(" key_id = ANY('%s') AND created_at >= %d AND created_at <= %d", sliceToSqlStringArray(keyIds), start, end)
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	events := []*event.Event{}
	rows, err := s.db.QueryContext(ctxTimeout, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return events, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var e event.Event
		var path sql.NullString
		var method sql.NullString
		var customId sql.NullString

		if err := rows.Scan(
			&e.Id,
			&e.CreatedAt,
			pq.Array(&e.Tags),
			&e.KeyId,
			&e.CostInUsd,
			&e.Provider,
			&e.Model,
			&e.Status,
			&e.PromptTokenCount,
			&e.CompletionTokenCount,
			&e.LatencyInMs,
			&path,
			&method,
			&customId,
			&e.Request,
			&e.Response,
			&e.UserId,
			&e.Action,
			&e.PolicyId,
			&e.RouteId,
			&e.CorrelationId,
			&e.Metadata,
		); err != nil {
			return nil, err
		}

		pe := &e
		pe.Path = path.String
		pe.Method = method.String
		pe.CustomId = customId.String

		events = append(events, pe)
	}

	return events, nil
}

func (s *Store) GetLatencyPercentiles(start, end int64, tags, keyIds []string) ([]float64, error) {
	eventSelectionBlock := `
	WITH events_table AS
		(
			SELECT * FROM events
	`

	conditionBlock := fmt.Sprintf("WHERE created_at >= %d AND created_at <= %d ", start, end)
	if len(tags) != 0 {
		conditionBlock += fmt.Sprintf("AND tags @> '%s' ", sliceToSqlStringArray(tags))
	}

	if len(keyIds) != 0 {
		conditionBlock += fmt.Sprintf("AND key_id = ANY('%s')", sliceToSqlStringArray(keyIds))
	}

	if len(tags) != 0 || len(keyIds) != 0 {
		eventSelectionBlock += conditionBlock
	}

	eventSelectionBlock += ")"

	query :=
		`
		SELECT    COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY events_table.latency_in_ms), 0) as median_latency, COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY events_table.latency_in_ms), 0) as top_latency
		FROM      events_table
		`

	query = eventSelectionBlock + query

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []float64{}
	for rows.Next() {
		var median float64
		var top float64

		if err := rows.Scan(
			&median,
			&top,
		); err != nil {
			return nil, err
		}

		data = []float64{
			median,
			top,
		}
	}

	return data, nil
}

func (s *Store) GetCustomIds(keyId string) ([]string, error) {
	query := fmt.Sprintf(`
	SELECT DISTINCT custom_id
	FROM events
	WHERE key_id = '%s' AND custom_id IS NOT NULL AND NOT custom_id = ''
	`, keyId)

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []string{}

	for rows.Next() {
		var customId string

		if err := rows.Scan(
			&customId,
		); err != nil {
			return nil, err
		}

		result = append(result, customId)
	}

	return result, nil
}

func (s *Store) GetUserIds(keyId string) ([]string, error) {
	query := fmt.Sprintf(`
	SELECT DISTINCT user_id
	FROM events
	WHERE key_id = '%s' AND NOT user_id = ''
	`, keyId)

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []string{}

	for rows.Next() {
		var userId string

		if err := rows.Scan(
			&userId,
		); err != nil {
			return nil, err
		}

		result = append(result, userId)
	}

	return result, nil
}

func (s *Store) GetTopKeyDataPoints(start, end int64, tags, keyIds []string, order string, limit, offset int, name string, revoked *bool) ([]*event.KeyDataPoint, error) {
	args := []any{}
	condition := ""
	condition2 := ""

	index := 1
	if len(tags) > 0 {
		condition += fmt.Sprintf("AND tags @> $%d", index)

		args = append(args, pq.Array(tags))
		index++
	}

	if len(keyIds) > 0 {
		condition += fmt.Sprintf(" AND key_id = ANY($%d)", index)

		args = append(args, pq.Array(keyIds))
		index++
	}

	if len(name) > 0 {
		condition += fmt.Sprintf(" AND LOWER(name) LIKE LOWER('%%%s%%')", name)
	}

	if revoked != nil {
		bools := "False"
		if *revoked {
			bools = "True"
		}

		condition += fmt.Sprintf(" AND revoked = %s", bools)
	}

	if len(tags) > 0 {
		condition2 += fmt.Sprintf("AND keys.tags @> $%d", index)

		args = append(args, pq.Array(tags))
		index++
	}

	if len(keyIds) > 0 {
		condition2 += fmt.Sprintf(" AND keys.key_id = ANY($%d)", index)

		args = append(args, pq.Array(keyIds))
	}

	if len(name) > 0 {
		condition2 += fmt.Sprintf(" AND LOWER(keys.name) LIKE LOWER('%%%s%%')", name)
	}

	if revoked != nil {
		bools := "False"
		if *revoked {
			bools = "True"
		}

		condition2 += fmt.Sprintf(" AND keys.revoked = %s", bools)
	}

	query := fmt.Sprintf(`
	WITH keys_table AS
	(
			SELECT key_id FROM keys WHERE created_at >= %d AND created_at < %d %s
	),top_keys_table AS
	(
		SELECT
		events.key_id,
		SUM(cost_in_usd) AS "CostInUsd"
		FROM events
		LEFT JOIN keys
		ON keys.key_id = events.key_id
		WHERE (events.key_id = '') IS FALSE AND events.created_at >= %d AND events.created_at < %d %s
		GROUP BY events.key_id
	)
	SELECT CASE
			WHEN top_keys_table.key_id IS NOT NULL THEN top_keys_table.key_id
			ELSE keys_table.key_id
		END
		AS key_id
  , COALESCE(top_keys_table."CostInUsd", 0) AS cost_in_usd
		FROM keys_table
		FULL JOIN top_keys_table
		ON top_keys_table.key_id = keys_table.key_id

`, start, end, condition, start, end, condition2)

	qorder := "DESC"
	if len(order) != 0 && strings.ToUpper(order) == "ASC" {
		qorder = "ASC"
	}

	query += fmt.Sprintf(`
	ORDER BY cost_in_usd %s
`, qorder)

	if limit != 0 {
		query += fmt.Sprintf(`
		LIMIT %d OFFSET %d;
	`, limit, offset)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []*event.KeyDataPoint{}
	for rows.Next() {
		var e event.KeyDataPoint
		var keyId sql.NullString

		additional := []any{
			&keyId,
			&e.CostInUsd,
		}

		if err := rows.Scan(
			additional...,
		); err != nil {
			return nil, err
		}

		pe := &e
		pe.KeyId = keyId.String

		data = append(data, pe)
	}

	return data, nil
}

func (s *Store) GetTopKeyRingDataPoints(start, end int64, tags []string, order string, limit, offset int, revoked *bool, topBy string) ([]*event.KeyRingDataPoint, error) {
	args := []any{}
	condition := ""
	condition2 := ""

	index := 1
	if len(tags) > 0 {
		condition += fmt.Sprintf("AND tags @> $%d", index)

		args = append(args, pq.Array(tags))
		index++
	}

	if revoked != nil {
		bools := "False"
		if *revoked {
			bools = "True"
		}

		condition += fmt.Sprintf(" AND revoked = %s", bools)
	}

	if len(tags) > 0 {
		condition2 += fmt.Sprintf("AND events.tags @> $%d", index)

		args = append(args, pq.Array(tags))
		index++
	}

	query := fmt.Sprintf(`
	WITH keys_table AS
	(
			SELECT key_id, key_ring FROM keys WHERE created_at >= %d AND created_at < %d %s
	),top_keys_table AS
	(
		SELECT
		key_ring,
		SUM(cost_in_usd) AS total_cost_in_usd,
		COUNT(*) AS total_requests
		FROM events
		LEFT JOIN keys
		ON keys.key_id = events.key_id
		WHERE (events.key_id = '') IS FALSE AND events.created_at >= %d AND events.created_at < %d %s
		GROUP BY key_ring
	)
	SELECT * FROM top_keys_table `, start, end, condition, start, end, condition2)

	qorder := "DESC"
	if len(order) != 0 && strings.ToUpper(order) == "ASC" {
		qorder = "ASC"
	}

	qtopBy := "total_cost_in_usd"
	if topBy != "" && slices.Contains(allowedTopBy, topBy) {
		qtopBy = topBy
	}
	query += fmt.Sprintf(`
	ORDER BY %s %s
`, qtopBy, qorder)

	if limit != 0 {
		query += fmt.Sprintf(`
		LIMIT %d OFFSET %d;
	`, limit, offset)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []*event.KeyRingDataPoint{}
	for rows.Next() {
		var e event.KeyRingDataPoint
		var keyRing sql.NullString

		additional := []any{
			&keyRing,
			&e.CostInUsd,
			&e.Requests,
		}

		if err := rows.Scan(
			additional...,
		); err != nil {
			return nil, err
		}

		pe := &e
		pe.KeyRing = keyRing.String

		data = append(data, pe)
	}

	return data, nil
}

func (s *Store) GetUsageData(tags []string) (*event.UsageData, error) {
	if len(tags) == 0 {
		return nil, internal_errors.NewValidationError("key reporting request tag cannot be empty")
	}
	condition := "tags @> $1 AND created_at > $2"
	nowTime := time.Now()
	sixMonthsAgo := nowTime.Add(-6 * 30 * 24 * time.Hour).Unix()
	dayAgo := nowTime.Add(-24 * time.Hour).Unix()
	weekAgo := nowTime.Add(-7 * 24 * time.Hour).Unix()
	monthAgo := nowTime.Add(-30 * 24 * time.Hour).Unix()

	args := []any{pq.Array(tags), sixMonthsAgo, dayAgo, weekAgo, monthAgo}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(cost_in_usd), 0) AS total_cost_in_usd,
			COALESCE(SUM(cost_in_usd) FILTER (WHERE created_at > $3), 0) AS total_cost_in_usd_last_day,
			COALESCE(SUM(cost_in_usd) FILTER (WHERE created_at > $4), 0) AS total_cost_in_usd_last_week,
			COALESCE(SUM(cost_in_usd) FILTER (WHERE created_at > $5), 0) AS total_cost_in_usd_last_month,
			COALESCE(COUNT(*), 0) AS total_requests,
			COALESCE(COUNT(*) FILTER (WHERE created_at > $3), 0) AS total_requests_last_day,
			COALESCE(COUNT(*) FILTER (WHERE created_at > $4), 0) AS total_requests_last_week,
			COALESCE(COUNT(*) FILTER (WHERE created_at > $5), 0) AS total_requests_last_month
		FROM events
		WHERE %s
	`, condition)

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	data := &event.UsageData{}
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&data.TotalUsage,
		&data.LastDayUsage,
		&data.LastWeekUsage,
		&data.LastMonthUsage,
		&data.TotalUsageRequests,
		&data.LastDayUsageRequests,
		&data.LastWeekUsageRequests,
		&data.LastMonthUsageRequests,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (s *Store) GetStatisticsData(level event.StatisticLevel, id *string) (*event.StatisticsData, error) {
	result := &event.StatisticsData{}

	switch level {
	case event.StisticLevels.All:
		total, err := s.getTotalCostPack(nil)
		if err != nil {
			return nil, err
		}

		orgPacks, err := s.GetOrgsCostPack()
		if err != nil {
			return nil, err
		}

		orgs := make([]event.ShortOrgStatisticsData, 0, len(orgPacks))
		for orgID, costs := range orgPacks {
			orgs = append(orgs, event.ShortOrgStatisticsData{Id: orgID, Costs: costs})
		}
		slices.SortFunc(orgs, func(a, b event.ShortOrgStatisticsData) int {
			return strings.Compare(a.Id, b.Id)
		})

		result.AllStatisticsData = &event.AllStatisticsData{
			Total: *total,
			Orgs:  orgs,
		}
	case event.StisticLevels.Org:
		if id == nil || len(*id) == 0 {
			return nil, internal_errors.NewValidationError("id must be provided when level is 'org'")
		}

		costs, err := s.getTotalCostPack([]string{orgTagPrefix + *id})
		if err != nil {
			return nil, err
		}

		coursePacks, err := s.GetCoursesCostPack(*id)
		if err != nil {
			return nil, err
		}

		courses := make([]event.ShortCourseStatisticsData, 0, len(coursePacks))
		for courseID, courseCosts := range coursePacks {
			courses = append(courses, event.ShortCourseStatisticsData{Id: courseID, Costs: courseCosts})
		}
		slices.SortFunc(courses, func(a, b event.ShortCourseStatisticsData) int {
			return strings.Compare(a.Id, b.Id)
		})

		dailySpecial, err := s.getDailyDistribution([]string{orgTagPrefix + *id}, codioSpecialTag)
		if err != nil {
			return nil, err
		}
		weeklySpecial, err := s.getPeriodDistribution([]string{orgTagPrefix + *id}, codioSpecialTag, "week")
		if err != nil {
			return nil, err
		}
		monthlySpecial, err := s.getPeriodDistribution([]string{orgTagPrefix + *id}, codioSpecialTag, "month")
		if err != nil {
			return nil, err
		}
		dailyProvided, err := s.getDailyDistribution([]string{orgTagPrefix + *id}, codioProvidedTag)
		if err != nil {
			return nil, err
		}
		weeklyProvided, err := s.getPeriodDistribution([]string{orgTagPrefix + *id}, codioProvidedTag, "week")
		if err != nil {
			return nil, err
		}
		monthlyProvided, err := s.getPeriodDistribution([]string{orgTagPrefix + *id}, codioProvidedTag, "month")
		if err != nil {
			return nil, err
		}
		topFive, err := s.getTopFiveUserSpends([]string{orgTagPrefix + *id})
		if err != nil {
			return nil, err
		}

		result.OrgStatisticsData = &event.OrgStatisticsData{
			Id:                               *id,
			Costs:                            *costs,
			Courses:                          courses,
			DailySpecialDistribution:         dailySpecial,
			WeeklySpecialDistribution:        weeklySpecial,
			MonthlySpecialDistribution:       monthlySpecial,
			DailyCodioProvidedDistribution:   dailyProvided,
			WeeklyCodioProvidedDistribution:  weeklyProvided,
			MonthlyCodioProvidedDistribution: monthlyProvided,
			TopFive:                          topFive,
		}
	case event.StisticLevels.Course:
		if id == nil || len(*id) == 0 {
			return nil, internal_errors.NewValidationError("id must be provided when level is 'course'")
		}

		costs, err := s.GetCourseCostPack(*id)
		if err != nil {
			return nil, err
		}

		dailySpecial, err := s.getDailyDistribution([]string{courseTagPrefix + *id}, codioSpecialTag)
		if err != nil {
			return nil, err
		}
		weeklySpecial, err := s.getPeriodDistribution([]string{courseTagPrefix + *id}, codioSpecialTag, "week")
		if err != nil {
			return nil, err
		}
		monthlySpecial, err := s.getPeriodDistribution([]string{courseTagPrefix + *id}, codioSpecialTag, "month")
		if err != nil {
			return nil, err
		}
		dailyProvided, err := s.getDailyDistribution([]string{courseTagPrefix + *id}, codioProvidedTag)
		if err != nil {
			return nil, err
		}
		weeklyProvided, err := s.getPeriodDistribution([]string{courseTagPrefix + *id}, codioProvidedTag, "week")
		if err != nil {
			return nil, err
		}
		monthlyProvided, err := s.getPeriodDistribution([]string{courseTagPrefix + *id}, codioProvidedTag, "month")
		if err != nil {
			return nil, err
		}
		topFive, err := s.getTopFiveUserSpends([]string{courseTagPrefix + *id})
		if err != nil {
			return nil, err
		}

		result.CourseStatisticsData = &event.CourseStatisticsData{
			Id:                               *id,
			Costs:                            *costs,
			DailySpecialDistribution:         dailySpecial,
			WeeklySpecialDistribution:        weeklySpecial,
			MonthlySpecialDistribution:       monthlySpecial,
			DailyCodioProvidedDistribution:   dailyProvided,
			WeeklyCodioProvidedDistribution:  weeklyProvided,
			MonthlyCodioProvidedDistribution: monthlyProvided,
			TopFive:                          topFive,
		}
	default:
		return nil, internal_errors.NewValidationError("invalid level")
	}

	return result, nil
}

const (
	orgTagPrefix          = "org-tag-"
	userTagPrefix         = "user-tag-"
	courseTagPrefix       = "course-tag-"
	codioSpecialTag       = "codio-special"
	codioProvidedTag      = "codio-provided"
	statisticsLookbackAge = -5 * 30 * 24 * time.Hour
)

func (s *Store) getGroupedCostPack(groupBy string, filterTags []string) (map[string]event.CostPack, error) {
	now := time.Now()
	oneMonthAgo := now.Add(-30 * 24 * time.Hour).Unix()
	fiveMonthsAgo := now.Add(-5 * 30 * 24 * time.Hour).Unix()

	args := []any{groupBy, oneMonthAgo, fiveMonthsAgo}
	conditions := []string{"e.created_at >= $3"}
	index := 4

	for _, tag := range filterTags {
		conditions = append(conditions, fmt.Sprintf("e.tags @> $%d", index))
		args = append(args, pq.Array([]string{tag}))
		index++
	}

	query := fmt.Sprintf(`
		SELECT
			regexp_replace(tag.tag, '^' || $1, '') AS grouped_id,
			COALESCE(SUM(e.cost_in_usd) FILTER (WHERE e.created_at >= $2 AND e.tags @> ARRAY['%s']::varchar[]), 0) AS codio_provided_one_month,
			COALESCE(SUM(e.cost_in_usd) FILTER (WHERE e.tags @> ARRAY['%s']::varchar[]), 0) AS codio_provided_five_month,
			COALESCE(SUM(e.cost_in_usd) FILTER (WHERE e.created_at >= $2 AND e.tags @> ARRAY['%s']::varchar[]), 0) AS codio_special_one_month,
			COALESCE(SUM(e.cost_in_usd) FILTER (WHERE e.tags @> ARRAY['%s']::varchar[]), 0) AS codio_special_five_month
		FROM events e,
		LATERAL unnest(e.tags) AS tag(tag)
		WHERE tag.tag LIKE $1 || '%%'
			AND %s
		GROUP BY regexp_replace(tag.tag, '^' || $1, '')
	`, codioProvidedTag, codioProvidedTag, codioSpecialTag, codioSpecialTag, strings.Join(conditions, " AND "))

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]event.CostPack)
	for rows.Next() {
		var id string
		var pack event.CostPack

		if err := rows.Scan(
			&id,
			&pack.CodioProvided.OneMonth,
			&pack.CodioProvided.FiveMonth,
			&pack.CodioSpecial.OneMonth,
			&pack.CodioSpecial.FiveMonth,
		); err != nil {
			return nil, err
		}

		result[id] = pack
	}

	return result, nil
}

func (s *Store) getTotalCostPack(filterTags []string) (*event.CostPack, error) {
	now := time.Now()
	oneMonthAgo := now.Add(-30 * 24 * time.Hour).Unix()
	fiveMonthsAgo := now.Add(-5 * 30 * 24 * time.Hour).Unix()

	args := []any{oneMonthAgo, fiveMonthsAgo}
	conditions := []string{"created_at >= $2"}
	index := 3

	for _, tag := range filterTags {
		conditions = append(conditions, fmt.Sprintf("tags @> $%d", index))
		args = append(args, pq.Array([]string{tag}))
		index++
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(cost_in_usd) FILTER (WHERE created_at >= $1 AND tags @> ARRAY['%s']::varchar[]), 0) AS codio_provided_one_month,
			COALESCE(SUM(cost_in_usd) FILTER (WHERE tags @> ARRAY['%s']::varchar[]), 0) AS codio_provided_five_month,
			COALESCE(SUM(cost_in_usd) FILTER (WHERE created_at >= $1 AND tags @> ARRAY['%s']::varchar[]), 0) AS codio_special_one_month,
			COALESCE(SUM(cost_in_usd) FILTER (WHERE tags @> ARRAY['%s']::varchar[]), 0) AS codio_special_five_month
		FROM events
		WHERE %s
	`, codioProvidedTag, codioProvidedTag, codioSpecialTag, codioSpecialTag, strings.Join(conditions, " AND "))

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	pack := &event.CostPack{}
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&pack.CodioProvided.OneMonth,
		&pack.CodioProvided.FiveMonth,
		&pack.CodioSpecial.OneMonth,
		&pack.CodioSpecial.FiveMonth,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pack, nil
		}
		return nil, err
	}

	return pack, nil
}

func (s *Store) GetOrgsCostPack() (map[string]event.CostPack, error) {
	return s.getGroupedCostPack(orgTagPrefix, nil)
}

func (s *Store) GetCoursesCostPack(orgId string) (map[string]event.CostPack, error) {
	return s.getGroupedCostPack(courseTagPrefix, []string{orgTagPrefix + orgId})
}

func (s *Store) GetCourseCostPack(courseId string) (*event.CostPack, error) {
	return s.getTotalCostPack([]string{courseTagPrefix + courseId})
}

func (s *Store) getDailyDistribution(filterTags []string, costTypeTag string) ([]event.DailySpendDistributionDataPoint, error) {
	start := beginningOfDay(time.Now().UTC()).AddDate(0, 0, -29)
	end := beginningOfDay(time.Now().UTC()).AddDate(0, 0, 1)

	rows, err := s.getPeriodKpisRows(filterTags, costTypeTag, "day", start, end)
	if err != nil {
		return nil, err
	}

	rowMap := make(map[string]financialKpiRow, len(rows))
	for _, row := range rows {
		rowMap[row.periodStart.Format("Jan-02")] = row
	}

	result := make([]event.DailySpendDistributionDataPoint, 0, 30)
	for current := start; current.Before(end); current = current.AddDate(0, 0, 1) {
		key := current.Format("Jan-02")
		row, ok := rowMap[key]
		if !ok {
			result = append(result, event.DailySpendDistributionDataPoint{Date: key})
			continue
		}
		result = append(result, event.DailySpendDistributionDataPoint{
			MaxUserSpend:    row.max,
			MedianUserSpend: row.median,
			AvgUserSpend:    row.avg,
			P95UserSpend:    row.p95,
			P99UserSpend:    row.p99,
			Date:            key,
		})
	}

	return result, nil
}

func (s *Store) getPeriodDistribution(filterTags []string, costTypeTag, period string) ([]event.PeriodSpendDistributionDataPoint, error) {
	var start time.Time
	var end time.Time

	now := time.Now().UTC()
	switch period {
	case "week":
		start = beginningOfWeek(now).AddDate(0, 0, -7*20)
		end = beginningOfWeek(now).AddDate(0, 0, 7)
	case "month":
		start = beginningOfMonth(now).AddDate(0, -4, 0)
		end = beginningOfMonth(now).AddDate(0, 1, 0)
	default:
		return nil, internal_errors.NewValidationError("unsupported period")
	}

	lookbackStart := time.Now().UTC().Add(statisticsLookbackAge)
	if start.Before(lookbackStart) {
		start = truncateToPeriodStart(lookbackStart, period)
	}

	rows, err := s.getPeriodKpisRows(filterTags, costTypeTag, period, start, end)
	if err != nil {
		return nil, err
	}

	rowMap := make(map[string]financialKpiRow, len(rows))
	for _, row := range rows {
		rowMap[formatPeriodLabel(row.periodStart, period)] = row
	}

	result := []event.PeriodSpendDistributionDataPoint{}
	for current := start; current.Before(end); current = addPeriod(current, period) {
		label := formatPeriodLabel(current, period)
		row, ok := rowMap[label]
		if !ok {
			result = append(result, event.PeriodSpendDistributionDataPoint{DatePeriod: label})
			continue
		}
		result = append(result, event.PeriodSpendDistributionDataPoint{
			MaxUserSpend:    row.max,
			MedianUserSpend: row.median,
			AvgUserSpend:    row.avg,
			P95UserSpend:    row.p95,
			P99UserSpend:    row.p99,
			DatePeriod:      label,
		})
	}

	return result, nil
}

type financialKpiRow struct {
	periodStart time.Time
	max         float64
	median      float64
	avg         float64
	p95         float64
	p99         float64
}

func (s *Store) getPeriodKpisRows(filterTags []string, costTypeTag, period string, start, end time.Time) ([]financialKpiRow, error) {
	args := []any{userTagPrefix, pq.Array([]string{costTypeTag}), start.Unix(), end.Unix()}
	conditions := []string{"tag.tag LIKE $1 || '%'", "e.tags @> $2", "e.created_at >= $3", "e.created_at < $4"}
	index := 5

	for _, tag := range filterTags {
		conditions = append(conditions, fmt.Sprintf("e.tags @> $%d", index))
		args = append(args, pq.Array([]string{tag}))
		index++
	}

	periodExpr := fmt.Sprintf("date_trunc('%s', timezone('UTC', to_timestamp(e.created_at)))", period)
	query := fmt.Sprintf(`
		WITH per_user_period AS (
			SELECT
				%s AS period_start,
				regexp_replace(tag.tag, '^' || $1, '') AS user_id,
				COALESCE(SUM(e.cost_in_usd), 0) AS user_period_cost
			FROM events e,
			LATERAL unnest(e.tags) AS tag(tag)
			WHERE %s
			GROUP BY %s, regexp_replace(tag.tag, '^' || $1, '')
		)
		SELECT
			period_start,
			COALESCE(MAX(user_period_cost), 0) AS max_user_spend,
			COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY user_period_cost), 0) AS median_user_spend,
			COALESCE(AVG(user_period_cost), 0) AS avg_user_spend,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY user_period_cost), 0) AS p95_user_spend,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY user_period_cost), 0) AS p99_user_spend
		FROM per_user_period
		GROUP BY period_start
		ORDER BY period_start
	`, periodExpr, strings.Join(conditions, " AND "), periodExpr)

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	queryRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer queryRows.Close()

	result := []financialKpiRow{}
	for queryRows.Next() {
		var row financialKpiRow
		if err := queryRows.Scan(&row.periodStart, &row.max, &row.median, &row.avg, &row.p95, &row.p99); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := queryRows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Store) getTopFiveUserSpends(filterTags []string) ([]event.TopFiveUserSpend, error) {
	start := beginningOfMonth(time.Now().UTC())
	end := beginningOfMonth(time.Now().UTC()).AddDate(0, 1, 0)

	args := []any{userTagPrefix, start.Unix(), end.Unix()}
	conditions := []string{"tag.tag LIKE $1 || '%'", "e.created_at >= $2", "e.created_at < $3"}
	index := 4

	for _, tag := range filterTags {
		conditions = append(conditions, fmt.Sprintf("e.tags @> $%d", index))
		args = append(args, pq.Array([]string{tag}))
		index++
	}

	query := fmt.Sprintf(`
		SELECT
			regexp_replace(tag.tag, '^' || $1, '') AS user_id,
			COALESCE(SUM(e.cost_in_usd), 0) AS spend_last_month
		FROM events e,
		LATERAL unnest(e.tags) AS tag(tag)
		WHERE %s
		GROUP BY regexp_replace(tag.tag, '^' || $1, '')
		ORDER BY spend_last_month DESC, user_id ASC
		LIMIT 5
	`, strings.Join(conditions, " AND "))

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []event.TopFiveUserSpend{}
	for rows.Next() {
		var row event.TopFiveUserSpend
		if err := rows.Scan(&row.UserId, &row.SpendLastMonth); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func beginningOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func beginningOfWeek(t time.Time) time.Time {
	dayStart := beginningOfDay(t)
	weekday := int(dayStart.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return dayStart.AddDate(0, 0, -(weekday - 1))
}

func beginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func truncateToPeriodStart(t time.Time, period string) time.Time {
	switch period {
	case "day":
		return beginningOfDay(t)
	case "week":
		return beginningOfWeek(t)
	case "month":
		return beginningOfMonth(t)
	default:
		return beginningOfDay(t)
	}
}

func addPeriod(t time.Time, period string) time.Time {
	switch period {
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

func formatPeriodLabel(start time.Time, period string) string {
	switch period {
	case "week":
		end := start.AddDate(0, 0, 6)
		return fmt.Sprintf("%s/%s", start.Format("Jan-02"), end.Format("Jan-02"))
	case "month":
		return start.Format("Jan")
	default:
		return start.Format("Jan-02")
	}
}

func (s *Store) GetAggregatedEventByDayDataPoints(start, end int64, keyIds []string) ([]*event.DataPointV2, error) {
	conditionBlock := fmt.Sprintf("WHERE time_stamp >= %d AND time_stamp < %d ", start, end)
	if len(keyIds) != 0 {
		conditionBlock += fmt.Sprintf("AND key_id = ANY('%s')", sliceToSqlStringArray(keyIds))
	}

	query := fmt.Sprintf(
		`
		SELECT * FROM event_agg_by_day
		%s
		ORDER BY  event_agg_by_day.time_stamp;
		`,
		conditionBlock,
	)

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []*event.DataPointV2{}
	for rows.Next() {
		var e event.DataPointV2
		var keyId sql.NullString
		var id sql.NullInt32

		additional := []any{
			&id,
			&e.TimeStamp,
			&e.NumberOfRequests,
			&e.CostInUsd,
			&e.LatencyInMs,
			&e.PromptTokenCount,
			&e.CompletionTokenCount,
			&e.SuccessCount,
			&keyId,
		}

		if err := rows.Scan(
			additional...,
		); err != nil {
			return nil, err
		}

		pe := &e
		pe.KeyId = keyId.String

		data = append(data, pe)
	}

	return data, nil
}

func (s *Store) GetEventDataPoints(start, end, increment int64, tags, keyIds, customIds, userIds []string, filters []string) ([]*event.DataPoint, error) {
	groupByQuery := "GROUP BY time_series_table.series"
	selectQuery := "SELECT series AS time_stamp, COALESCE(COUNT(events_table.event_id),0) AS num_of_requests, COALESCE(SUM(events_table.cost_in_usd),0) AS cost_in_usd, COALESCE(SUM(events_table.latency_in_ms),0) AS latency_in_ms, COALESCE(SUM(events_table.prompt_token_count),0) AS prompt_token_count, COALESCE(SUM(events_table.completion_token_count),0) AS completion_token_count, COALESCE(SUM(CASE WHEN status_code = 200 THEN 1 END),0) AS success_count"

	if len(filters) != 0 {
		for _, filter := range filters {
			if filter == "model" {
				groupByQuery += ",events_table.model"
				selectQuery += ",events_table.model as model"
			}

			if filter == "keyId" {
				groupByQuery += ",events_table.key_id"
				selectQuery += ",events_table.key_id as keyId"
			}

			if filter == "customId" {
				groupByQuery += ",events_table.custom_id"
				selectQuery += ",events_table.custom_id as customId"
			}

			if filter == "userId" {
				groupByQuery += ",events_table.user_id"
				selectQuery += ",events_table.user_id as userId"
			}
		}
	}

	query := fmt.Sprintf(
		`
		,time_series_table AS
		(
			SELECT generate_series(%d, %d, %d) series
		)
		%s
		FROM       time_series_table
		LEFT JOIN  events_table
		ON         events_table.created_at >= time_series_table.series
		AND        events_table.created_at < time_series_table.series + %d
		%s
		ORDER BY  time_series_table.series;
		`,
		start, end, increment, selectQuery, increment, groupByQuery,
	)

	eventSelectionBlock := `
	WITH events_table AS
		(
			SELECT * FROM events
	`

	conditionBlock := fmt.Sprintf("WHERE created_at >= %d AND created_at < %d ", start, end)
	if len(tags) != 0 {
		conditionBlock += fmt.Sprintf("AND tags @> '%s' ", sliceToSqlStringArray(tags))
	}

	if len(keyIds) != 0 {
		conditionBlock += fmt.Sprintf("AND key_id = ANY('%s')", sliceToSqlStringArray(keyIds))
	}

	if len(customIds) != 0 {
		conditionBlock += fmt.Sprintf("AND custom_id = ANY('%s')", sliceToSqlStringArray(customIds))
	}

	if len(userIds) != 0 {
		conditionBlock += fmt.Sprintf("AND user_id = ANY('%s')", sliceToSqlStringArray(userIds))
	}

	eventSelectionBlock += conditionBlock
	eventSelectionBlock += ")"

	query = eventSelectionBlock + query

	ctx, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := []*event.DataPoint{}
	for rows.Next() {
		var e event.DataPoint
		var model sql.NullString
		var keyId sql.NullString
		var customId sql.NullString
		var userId sql.NullString

		additional := []any{
			&e.TimeStamp,
			&e.NumberOfRequests,
			&e.CostInUsd,
			&e.LatencyInMs,
			&e.PromptTokenCount,
			&e.CompletionTokenCount,
			&e.SuccessCount,
		}

		if len(filters) != 0 {
			for _, filter := range filters {
				if filter == "model" {
					additional = append(additional, &model)
				}

				if filter == "keyId" {
					additional = append(additional, &keyId)
				}

				if filter == "customId" {
					additional = append(additional, &customId)
				}

				if filter == "userId" {
					additional = append(additional, &userId)
				}
			}
		}

		if err := rows.Scan(
			additional...,
		); err != nil {
			return nil, err
		}

		pe := &e
		pe.Model = model.String
		pe.KeyId = keyId.String
		pe.CustomId = customId.String
		pe.UserId = userId.String

		data = append(data, pe)
	}

	return data, nil
}

func (s *Store) GetEventsV2(req *event.EventRequest) (*event.EventResponse, error) {
	query := fmt.Sprintf(`
		SELECT * FROM events WHERE created_at >= %d AND created_at < %d
	`, req.Start, req.End)

	cquery := fmt.Sprintf(`
	SELECT COUNT(*) FROM events WHERE created_at >= %d AND created_at < %d
`, req.Start, req.End)

	if len(req.UserIds) != 0 {
		query += fmt.Sprintf(" AND user_id = ANY('%s')", sliceToSqlStringArray(req.UserIds))
		cquery += fmt.Sprintf(" AND user_id = ANY('%s')", sliceToSqlStringArray(req.UserIds))
	}

	if req.Status != 0 {
		query += fmt.Sprintf(" AND status_code = %d", req.Status)
		cquery += fmt.Sprintf(" AND status_code = %d", req.Status)
	}

	if len(req.CustomIds) != 0 {
		query += fmt.Sprintf(" AND custom_id = ANY('%s')", sliceToSqlStringArray(req.CustomIds))
		cquery += fmt.Sprintf(" AND custom_id = ANY('%s')", sliceToSqlStringArray(req.CustomIds))
	}

	if len(req.KeyIds) != 0 {
		query += fmt.Sprintf(" AND key_id = ANY('%s')", sliceToSqlStringArray(req.KeyIds))
		cquery += fmt.Sprintf(" AND key_id = ANY('%s')", sliceToSqlStringArray(req.KeyIds))
	}

	if len(req.Tags) != 0 {
		query += fmt.Sprintf(" AND tags @> '%s'", sliceToSqlStringArray(req.Tags))
		cquery += fmt.Sprintf(" AND tags @> '%s'", sliceToSqlStringArray(req.Tags))
	}

	if len(req.PolicyIds) != 0 {
		query += fmt.Sprintf(" AND policy_id = ANY('%s')", sliceToSqlStringArray(req.PolicyIds))
		cquery += fmt.Sprintf(" AND policy_id = ANY('%s')", sliceToSqlStringArray(req.PolicyIds))
	}

	if len(req.Actions) != 0 {
		query += fmt.Sprintf(" AND action = ANY('%s')", sliceToSqlStringArray(req.Actions))
		cquery += fmt.Sprintf(" AND action = ANY('%s')", sliceToSqlStringArray(req.Actions))
	}

	if len(req.CostOrder) != 0 {
		query += fmt.Sprintf(" ORDER BY cost_in_usd %s", strings.ToUpper(req.CostOrder))
	}

	if len(req.DateOrder) != 0 {
		query += fmt.Sprintf(" ORDER BY created_at %s", strings.ToUpper(req.DateOrder))
	}

	if req.Limit != 0 {
		query += fmt.Sprintf(` LIMIT %d OFFSET %d;`, req.Limit, req.Offset)
	}

	qrContext, qrCancel := context.WithTimeout(context.Background(), s.rt)
	defer qrCancel()

	resp := &event.EventResponse{}

	if req.ReturnCount {
		count := 0
		err := s.db.QueryRowContext(qrContext, cquery).Scan(&count)
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
		}

		resp.Count = count
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()

	events := []*event.Event{}
	rows, err := s.db.QueryContext(ctxTimeout, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var e event.Event
		var path sql.NullString
		var method sql.NullString
		var customId sql.NullString

		if err := rows.Scan(
			&e.Id,
			&e.CreatedAt,
			pq.Array(&e.Tags),
			&e.KeyId,
			&e.CostInUsd,
			&e.Provider,
			&e.Model,
			&e.Status,
			&e.PromptTokenCount,
			&e.CompletionTokenCount,
			&e.LatencyInMs,
			&path,
			&method,
			&customId,
			&e.Request,
			&e.Response,
			&e.UserId,
			&e.Action,
			&e.PolicyId,
			&e.RouteId,
			&e.CorrelationId,
			&e.Metadata,
		); err != nil {
			return nil, err
		}

		pe := &e
		pe.Path = path.String
		pe.Method = method.String
		pe.CustomId = customId.String

		events = append(events, pe)
	}

	resp.Events = events

	return resp, nil
}

func isJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

func (s *Store) InsertEvent(e *event.Event) error {
	rawResponse := string(e.Response)
	if !isJSON(rawResponse) {
		e.Response = []byte(`{"bricksError": "response is not a valid json"}`)
	}

	query := `
		INSERT INTO events (event_id, created_at, tags, key_id, cost_in_usd, provider, model, status_code, prompt_token_count, completion_token_count, latency_in_ms, path, method, custom_id, request, response, user_id, action, policy_id, route_id, correlation_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`

	values := []any{
		e.Id,
		e.CreatedAt,
		sliceToSqlStringArray(e.Tags),
		e.KeyId,
		e.CostInUsd,
		e.Provider,
		e.Model,
		e.Status,
		e.PromptTokenCount,
		e.CompletionTokenCount,
		e.LatencyInMs,
		e.Path,
		e.Method,
		e.CustomId,
		e.Request,
		e.Response,
		e.UserId,
		e.Action,
		e.PolicyId,
		e.RouteId,
		e.CorrelationId,
		e.Metadata,
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, query, values...); err != nil {
		return err
	}

	return nil
}
