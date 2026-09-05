package main

import (
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/bricks-cloud/bricksllm/internal/storage/postgresql"
)

const (
	mockGeneratedTag = "mock-generated"
	orgTagPrefix     = "org-tag-"
	courseTagPrefix  = "course-tag-"
	userTagPrefix    = "user-tag-"
	codioSpecialTag  = "codio-special"
	codioProvidedTag = "codio-provided"
)

type config struct {
	dsn            string
	orgCount       int
	coursesPerOrg  int
	usersPerCourse int
	days           int
	seed           int64
	verbose        bool
}

type courseDef struct {
	orgID    string
	courseID string
	users    []string
}

func main() {
	cfg := parseFlags()

	store, err := postgresql.NewStore(cfg.dsn, 30*time.Second, 30*time.Second)
	if err != nil {
		panic(err)
	}

	if err := store.CreateEventsTable(); err != nil {
		panic(err)
	}
	if err := store.AlterEventsTable(); err != nil {
		panic(err)
	}

	db, err := sql.Open("postgres", cfg.dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rng := rand.New(rand.NewSource(cfg.seed))
	courses := buildTopology(cfg)

	inserted, err := seedEvents(db, rng, cfg, courses)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Inserted %d mock events into events table.\n", inserted)
	fmt.Printf("All generated rows include tag %q.\n", mockGeneratedTag)
	fmt.Printf("Cleanup SQL: DELETE FROM events WHERE tags @> ARRAY['%s']::varchar[];\n", mockGeneratedTag)
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.dsn, "dsn", "postgresql:///?sslmode=disable&user=postgres&password=postgres&host=localhost&port=5432", "PostgreSQL DSN")
	flag.IntVar(&cfg.orgCount, "orgs", 3, "Number of organizations")
	flag.IntVar(&cfg.coursesPerOrg, "courses-per-org", 4, "Number of courses per organization")
	flag.IntVar(&cfg.usersPerCourse, "users-per-course", 80, "Number of users per course")
	flag.IntVar(&cfg.days, "days", 150, "How many past days of events to generate")
	flag.Int64Var(&cfg.seed, "seed", 42, "Random seed for reproducible data")
	flag.BoolVar(&cfg.verbose, "verbose", false, "Print progress details")
	flag.Parse()

	if cfg.orgCount <= 0 || cfg.coursesPerOrg <= 0 || cfg.usersPerCourse <= 0 || cfg.days <= 0 {
		panic("orgs, courses-per-org, users-per-course and days must be positive")
	}

	return cfg
}

func buildTopology(cfg config) []courseDef {
	courses := make([]courseDef, 0, cfg.orgCount*cfg.coursesPerOrg)
	for orgIdx := 1; orgIdx <= cfg.orgCount; orgIdx++ {
		orgID := fmt.Sprintf("mock-org-%02d", orgIdx)
		for courseIdx := 1; courseIdx <= cfg.coursesPerOrg; courseIdx++ {
			courseID := fmt.Sprintf("mock-course-%02d-%02d", orgIdx, courseIdx)
			users := make([]string, 0, cfg.usersPerCourse)
			for userIdx := 1; userIdx <= cfg.usersPerCourse; userIdx++ {
				users = append(users, fmt.Sprintf("mock-user-%02d-%02d-%03d", orgIdx, courseIdx, userIdx))
			}
			courses = append(courses, courseDef{orgID: orgID, courseID: courseID, users: users})
		}
	}
	return courses
}

func seedEvents(db *sql.DB, rng *rand.Rand, cfg config, courses []courseDef) (int, error) {
	const insertQuery = `
		INSERT INTO events (
			event_id,
			created_at,
			tags,
			key_id,
			cost_in_usd,
			provider,
			model,
			status_code,
			prompt_token_count,
			completion_token_count,
			latency_in_ms,
			path,
			method,
			custom_id,
			request,
			response,
			user_id,
			action,
			policy_id,
			route_id,
			correlation_id,
			metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
	`

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(insertQuery)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	inserted := 0

	for dayOffset := 0; dayOffset < cfg.days; dayOffset++ {
		day := now.AddDate(0, 0, -dayOffset)
		for _, course := range courses {
			activityMultiplier := courseActivityMultiplier(course)
			for userIndex, userID := range course.users {
				if !isUserActiveOnDay(rng, dayOffset, activityMultiplier, userIndex) {
					continue
				}

				eventCount := sampleEventCount(rng, dayOffset, activityMultiplier, userIndex)
				for eventIdx := 0; eventIdx < eventCount; eventIdx++ {
					createdAt := sampleTimestamp(rng, day)
					costTypeTag, cost := sampleCostProfile(rng, dayOffset, activityMultiplier, userIndex)
					promptTokens, completionTokens := sampleTokenCounts(rng, costTypeTag, activityMultiplier)
					latency := 200 + rng.Intn(1800)
					tags := []string{
						orgTagPrefix + course.orgID,
						courseTagPrefix + course.courseID,
						userTagPrefix + userID,
						costTypeTag,
						mockGeneratedTag,
					}

					keyID := fmt.Sprintf("mock-key-%s", course.orgID)
					customID := fmt.Sprintf("mock-run-%s", day.Format("20060102"))
					action := "mock-generate-statistics"
					policyID := fmt.Sprintf("mock-policy-%s", course.orgID)
					routeID := fmt.Sprintf("mock-route-%s", course.courseID)
					correlationID := uuid.NewString()
					requestJSON := fmt.Sprintf(`{"mock":true,"tag":"%s","courseId":"%s","userId":"%s"}`,
						mockGeneratedTag,
						course.courseID,
						userID,
					)
					responseJSON := `{"ok":true}`
					metadataJSON := fmt.Sprintf(`{"generator":"mock-data/generate_statistics_mock.go","dayOffset":%d}`, dayOffset)

					if _, err := stmt.Exec(
						uuid.NewString(),
						createdAt.Unix(),
						pq.Array(tags),
						keyID,
						cost,
						"openai",
						sampleModel(costTypeTag),
						200,
						promptTokens,
						completionTokens,
						latency,
						"/api/providers/openai/v1/responses",
						"POST",
						customID,
						requestJSON,
						responseJSON,
						userID,
						action,
						policyID,
						routeID,
						correlationID,
						metadataJSON,
					); err != nil {
						return inserted, err
					}
					inserted++
				}
			}
		}

		if cfg.verbose && dayOffset%7 == 0 {
			fmt.Printf("generated through day offset %d (%s), inserted=%d\n", dayOffset, day.Format("2006-01-02"), inserted)
		}
	}

	if err := tx.Commit(); err != nil {
		return inserted, err
	}

	return inserted, nil
}

func courseActivityMultiplier(course courseDef) float64 {
	switch {
	case strings.HasSuffix(course.courseID, "01"):
		return 0.8
	case strings.HasSuffix(course.courseID, "02"):
		return 1.0
	case strings.HasSuffix(course.courseID, "03"):
		return 1.3
	default:
		return 1.6
	}
}

func isUserActiveOnDay(rng *rand.Rand, dayOffset int, multiplier float64, userIndex int) bool {
	base := 0.25 + multiplier*0.18
	if userIndex < 5 {
		base += 0.20
	}
	if userIndex > 50 {
		base -= 0.08
	}
	if dayOffset%7 == 0 || dayOffset%7 == 6 {
		base *= 0.75
	}
	if dayOffset > 120 {
		base *= 0.90
	}
	if base > 0.95 {
		base = 0.95
	}
	if base < 0.05 {
		base = 0.05
	}
	return rng.Float64() < base
}

func sampleEventCount(rng *rand.Rand, dayOffset int, multiplier float64, userIndex int) int {
	count := 1
	if rng.Float64() < 0.45*multiplier {
		count++
	}
	if rng.Float64() < 0.15*multiplier {
		count += 1 + rng.Intn(2)
	}
	if userIndex < 3 && rng.Float64() < 0.20 {
		count += 2 + rng.Intn(4)
	}
	if dayOffset%30 == 0 && rng.Float64() < 0.30 {
		count += 2
	}
	if count > 10 {
		count = 10
	}
	return count
}

func sampleTimestamp(rng *rand.Rand, day time.Time) time.Time {
	base := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(rng.Intn(24)) * time.Hour).
		Add(time.Duration(rng.Intn(60)) * time.Minute).
		Add(time.Duration(rng.Intn(60)) * time.Second)
}

func sampleCostProfile(rng *rand.Rand, dayOffset int, multiplier float64, userIndex int) (string, float64) {
	costTypeTag := codioProvidedTag
	if rng.Float64() < 0.28 {
		costTypeTag = codioSpecialTag
	}

	base := 0.08 + rng.Float64()*0.90
	if costTypeTag == codioSpecialTag {
		base *= 1.7
	}
	base *= multiplier

	if userIndex < 3 {
		base *= 2.5
	}
	if userIndex >= 3 && userIndex < 10 {
		base *= 1.4
	}

	if dayOffset%14 == 0 && rng.Float64() < 0.18 {
		base *= 4.0 + rng.Float64()*5.0
	}
	if dayOffset%45 == 0 && rng.Float64() < 0.10 {
		base *= 8.0 + rng.Float64()*8.0
	}

	return costTypeTag, round2(base)
}

func sampleTokenCounts(rng *rand.Rand, costTypeTag string, multiplier float64) (int, int) {
	prompt := 300 + rng.Intn(2200)
	completion := 150 + rng.Intn(1800)
	if costTypeTag == codioSpecialTag {
		prompt = int(float64(prompt) * (1.2 + multiplier*0.2))
		completion = int(float64(completion) * (1.2 + multiplier*0.2))
	}
	return prompt, completion
}

func sampleModel(costTypeTag string) string {
	if costTypeTag == codioSpecialTag {
		return "gpt-5.4-mini"
	}
	return "gpt-5.4-nano"
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
