{
  "course_id": "course_ai_lit_2026",
  "course_name": "AI Literacy Basics",
  "org_id": "org_skolkovo_01",
  "currency": "USD",
  "time_frame": {
    "start": "2026-08-01T00:00:00Z",
    "end": "2026-08-31T23:59:59Z"
  },
  "financial_kpis": {
    "total_course_spend": 2450.75,
    "max_user_spend": 145.20,
    "median_user_spend": 18.50,
    "avg_user_spend": 24.02
  },
  "bell_curve_data": [
    { "cost_bucket_usd": "0-5", "user_count": 15 },
    { "cost_bucket_usd": "5-15", "user_count": 45 },
    { "cost_bucket_usd": "15-25", "user_count": 120 },
    { "cost_bucket_usd": "25-40", "user_count": 85 },
    { "cost_bucket_usd": "40-70", "user_count": 30 },
    { "cost_bucket_usd": "70-100", "user_count": 8 },
    { "cost_bucket_usd": "100+", "user_count": 2 }
  ],
  "users_table": [
    {
      "user_id": "user_student_442",
      "name": "Иван Иванов",
      "total_requests": 340,
      "total_spend": 18.45,
      "budget_limit_usd": 50.00,
      "limit_used_percentage": 36.9
    },
    {
      "user_id": "user_student_999",
      "name": "Алексей Петров (Аномалия)",
      "total_requests": 4500,
      "total_spend": 145.20,
      "budget_limit_usd": 150.00,
      "limit_used_percentage": 96.8
    }
  ]
}

-----------------------------------

statistics API response notes:
- all period-based distributions and KPIs below are calculated only from data not older than the last 5 months
- `costs.1month` and `costs.5month` remain cumulative spend windows
- period distributions are built from aggregated `user-day`, `user-week`, and `user-month` spend values

cost:
{
  "1month": float,
  "5month": float
}

costPack:
{
  "codioProvided": cost,
  "codioSpecial": cost
}

bellCurveDataPoint:
{
  "costBucketUsd": string,
  "sampleCount": int,
  "serialNo": int
}

financialKpis:
{
  "maxUserSpend": float,
  "medianUserSpend": float,
  "avgUserSpend": float,
  "p90UserSpend": float,
  "p95UserSpend": float,
  "p99UserSpend": float,
  "sampleCount": int,
  "recommendedSoftLimitUsd": float,
  "recommendedHardLimitUsd": float
}

spendPeriodStatistics:
{
  "bellCurve": [bellCurveDataPoint],
  "financialKpis": financialKpis
}

-----------------------

all:
{
  "total": costPack,
  "orgs": [
    {
      "id": string,
      "costs": costPack
    }
  ]
}

---------------------

org:
{
  "id": string,
  "costs": costPack,
  "courses": [
    {
      "id": string,
      "costs": costPack
    }
  ],
  "dailySpecial": spendPeriodStatistics,
  "weeklySpecial": spendPeriodStatistics,
  "monthlySpecial": spendPeriodStatistics
}

Recommended buckets for org special:
- daily:   0-1, 1-3, 3-5, 5-10, 10-20, 20+
- weekly:  0-5, 5-10, 10-20, 20-50, 50-100, 100+
- monthly: 0-10, 10-25, 25-50, 50-100, 100-250, 250+

---------------------

course:
{
  "id": string,
  "costs": costPack,
  "dailyCodioProvided": spendPeriodStatistics,
  "weeklyCodioProvided": spendPeriodStatistics,
  "monthlyCodioProvided": spendPeriodStatistics
}

Recommended buckets for course codio-provided:
- daily:   0-1, 1-3, 3-5, 5-10, 10-20, 20+
- weekly:  0-5, 5-10, 10-20, 20-50, 50-100, 100+
- monthly: 0-10, 10-25, 25-50, 50-100, 100-250, 250+

-----------------------------

How to interpret the returned statistics

General principles:
- all `daily*`, `weekly*`, and `monthly*` blocks are calculated only from events from the last 5 months
- each graph is built from aggregated spend per user per period, not from lifetime spend
- the backend first computes:
  - one value per `user-day`
  - one value per `user-week`
  - one value per `user-month`
- then it builds distributions and KPIs from those values

What `costs` means:
- `costs.codioProvided.1month` — cumulative spend for codio-provided traffic over the last 1 month
- `costs.codioProvided.5month` — cumulative spend for codio-provided traffic over the last 5 months
- `costs.codioSpecial.1month` — cumulative spend for codio-special traffic over the last 1 month
- `costs.codioSpecial.5month` — cumulative spend for codio-special traffic over the last 5 months
- this block is useful for total budget visibility, but by itself is not enough for choosing period-based limits

What `spendPeriodStatistics` means:
- `bellCurve` shows the distribution of spend samples for a specific period
- `financialKpis` shows summary statistics over the same sample set
- `sampleCount` is the number of aggregated samples used for that period

Example:
- if `dailySpecial.financialKpis.sampleCount = 1200`
- this means the dataset contains 1200 `user-day` spend values
- not necessarily 1200 unique users
- one active user can contribute multiple daily samples on different days

How to read the bell curve / histogram

For org-level `dailySpecial`:
- every sample is one user's total `codio-special` spend within one calendar day
- bucket `0-1` means: number of user-days where spend was less than 1 USD
- bucket `1-3` means: number of user-days where spend was from 1 USD to under 3 USD
- bucket `20+` means: number of user-days where spend was 20 USD or more

For course-level `weeklyCodioProvided`:
- every sample is one user's total `codio-provided` spend within one calendar week
- if the rightmost buckets are growing, users increasingly hit high weekly cost ranges

What the graph tells you:
- left-heavy distribution usually means most user-periods are cheap
- a long right tail means there are rare but expensive spikes
- if the center of mass shifts right over time, existing limits may become too strict
- if a large share of samples already sits near your current limit, many normal users may get blocked

What each KPI means

For every period block (`daily*`, `weekly*`, `monthly*`):
- `maxUserSpend` — the largest spend observed in one user-period sample
- `medianUserSpend` — the middle spend value; 50% of samples are below it
- `avgUserSpend` — arithmetic average across all samples
- `p90UserSpend` — 90% of samples are at or below this value
- `p95UserSpend` — 95% of samples are at or below this value
- `p99UserSpend` — 99% of samples are at or below this value
- `recommendedSoftLimitUsd` — current recommended soft threshold, equal to `p95`
- `recommendedHardLimitUsd` — current recommended hard threshold, equal to `p99 * 1.2`

How to use these metrics for limits

Daily limit:
- use `daily*` blocks
- good starting point:
  - soft limit = `p95`
  - hard limit = `p99 * 1.2`
- meaning:
  - soft limit should catch unusually expensive daily behavior while affecting only a small minority of user-days
  - hard limit should catch only strong outliers or suspicious spikes

Weekly limit:
- use `weekly*` blocks
- useful when daily usage is noisy, but you want to control total weekly burn
- if weekly p95 is stable but daily max is noisy, a weekly limit may be more product-friendly than a strict daily cap

Monthly limit:
- use `monthly*` blocks
- useful for budget governance and subscription-style controls
- if monthly p95 is close to business budget expectations, it is a strong candidate for the default monthly allowance

Practical recommendation flow

1. Start with daily statistics:
- inspect `daily*` bell curve
- check where most samples are concentrated
- compare current or planned daily limit with `p95` and `p99`

2. Check weekly statistics:
- confirm that users with many moderate daily sessions do not accumulate into unexpectedly high weekly totals
- if weekly tail is too wide, add a weekly limit even if daily looks acceptable

3. Check monthly statistics:
- make sure the monthly allowance matches your budget model
- monthly limit should reflect normal heavy users, not only median users

4. Review tails:
- if `maxUserSpend` is much larger than `p99UserSpend`, the system has extreme outliers
- such cases often justify a hard limit, anomaly alert, or additional investigation

Suggested interpretation patterns

Pattern A: Most samples in the first buckets, low p95, very high max
- normal usage is cheap
- there are rare spikes or outliers
- recommended action:
  - keep soft limit near p95
  - keep hard limit significantly above soft limit
  - investigate top offenders separately

Pattern B: Distribution centered close to the upper buckets
- many users naturally consume a lot
- low limits will likely block legitimate traffic
- recommended action:
  - increase default limits
  - use weekly/monthly controls instead of aggressive daily limits

Pattern C: Daily looks healthy, weekly/monthly tails are wide
- individual days are fine, but sustained usage is expensive
- recommended action:
  - keep daily limit moderate
  - add stronger weekly/monthly guardrails

Pattern D: p95 and p99 are both moving upward over time
- usage pattern is structurally changing
- recommended action:
  - review limits periodically
  - consider time-series tracking for p95/p99 as the next dashboard enhancement

Limitations of the current model

- the current API returns aggregated distributions and KPIs, not raw per-user samples
- recommendations are heuristic, not policy decisions
- `recommendedSoftLimitUsd` and `recommendedHardLimitUsd` are good defaults, but should still be reviewed against business rules
- if sample count is low, percentiles can be noisy; in that case, org-wide defaults or manual review may be safer

Suggested UI usage

For each org or course, show three sections:
- Daily
- Weekly
- Monthly

Inside each section show:
- histogram from `bellCurve`
- KPI cards:
  - median
  - p95
  - p99
  - max
  - recommended soft limit
  - recommended hard limit
- optional helper text:
  - "95% of user-periods are below X USD"
  - "Only 1% of user-periods exceed Y USD"

This makes the dashboard directly actionable for tuning quotas and budget limits.
