statistics API response notes:
- all statistics below use only data not older than the last 5 months
- `costs.1month` and `costs.5month` remain cumulative spend windows
- distributions are now time-series points, not histograms
- each point is calculated from unique user spend aggregated inside a calendar period
- calendar boundaries are used:
  - day: from start to end of day
  - week: from start to end of ISO-style week (Monday to Sunday)
  - month: from start to end of month
- returned windows:
  - `daily*Distribution`: one point per day for the last month
  - `weekly*Distribution`: one point per week for the last 5 months
  - `monthly*Distribution`: one point per month for the last 5 months

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

dailyDistributionDataPoint:
{
  "maxUserSpend": float,
  "medianUserSpend": float,
  "avgUserSpend": float,
  "p95UserSpend": float,
  "p99UserSpend": float,
  "date": "YYYY-MM-DD"
}

periodDistributionDataPoint:
{
  "maxUserSpend": float,
  "medianUserSpend": float,
  "avgUserSpend": float,
  "p95UserSpend": float,
  "p99UserSpend": float,
  "datePeriod": string
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

topFiveItem:
{
  "userId": string,
  "spendLastMonth": float
}

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
  "dailySpecialDistribution": [dailyDistributionDataPoint],
  "weeklySpecialDistribution": [periodDistributionDataPoint],
  "monthlySpecialDistribution": [periodDistributionDataPoint],
  "dailyCodioProvidedDistribution": [dailyDistributionDataPoint],
  "weeklyCodioProvidedDistribution": [periodDistributionDataPoint],
  "monthlyCodioProvidedDistribution": [periodDistributionDataPoint],
  "topFive": [topFiveItem]
}

---------------------

course:
{
  "id": string,
  "costs": costPack,
  "dailySpecialDistribution": [dailyDistributionDataPoint],
  "weeklySpecialDistribution": [periodDistributionDataPoint],
  "monthlySpecialDistribution": [periodDistributionDataPoint],
  "dailyCodioProvidedDistribution": [dailyDistributionDataPoint],
  "weeklyCodioProvidedDistribution": [periodDistributionDataPoint],
  "monthlyCodioProvidedDistribution": [periodDistributionDataPoint],
  "topFive": [topFiveItem]
}

-----------------------------

How to interpret the returned statistics

General principles:
- each point is calculated from unique users active in that calendar period
- for one period, each user contributes one aggregated spend value
- metrics are then calculated across the set of user totals for that period
- if a period has no activity, the API still returns a point with zero values so the frontend can render a continuous series

What `costs` means:
- `costs.codioProvided.1month` — cumulative spend for codio-provided traffic over the last 1 month
- `costs.codioProvided.5month` — cumulative spend for codio-provided traffic over the last 5 months
- `costs.codioSpecial.1month` — cumulative spend for codio-special traffic over the last 1 month
- `costs.codioSpecial.5month` — cumulative spend for codio-special traffic over the last 5 months
- this block is useful for total budget visibility, while the distribution arrays are useful for trend analysis

What each distribution point means:
- `maxUserSpend` — the highest per-user spend in that day/week/month
- `medianUserSpend` — 50% of active users in that period spent at or below this value
- `avgUserSpend` — arithmetic average per-user spend in that period
- `p95UserSpend` — 95% of active users in that period spent at or below this value
- `p99UserSpend` — 99% of active users in that period spent at or below this value
- `date` is used for daily points
- `datePeriod` is used for weekly/monthly points
- `topFive` is a companion list for the current scope and contains the top 5 users by spend for the last month

Date labels:
- daily uses `YYYY-MM-DD`
- weekly uses `YYYY-MM-DD/YYYY-MM-DD` where the label represents week start and week end
- monthly uses `YYYY-MM`

Examples:
- `dailySpecialDistribution[i]` shows one calendar day from the last month
- `weeklySpecialDistribution[i]` shows one calendar week from the last 5 months
- `monthlyCodioProvidedDistribution[i]` shows one calendar month from the last 5 months
- `topFive[i]` shows one of the five users with the highest spend for the last month in the current org or course scope

How to read `topFive`

- `topFive` is calculated for the last month
- it is scoped to the current entity:
  - for org statistics, top users inside that organization
  - for course statistics, top users inside that course
- it is not split by `codio-special` or `codio-provided`; it reflects total spend in scope for the last month
- it is useful for quickly identifying the most expensive users for investigation, outreach, or manual policy review

How to read the charts

Daily chart:
- use it to see short-term volatility and spikes
- `maxUserSpend` highlights strongest single-user bursts in a day
- `medianUserSpend` and `avgUserSpend` show whether broad usage is rising or only a few users are spiking

Weekly chart:
- use it to see medium-term usage stabilization
- compare weekly `p95UserSpend` and `p99UserSpend` across weeks
- this is useful for tuning weekly guardrails

Monthly chart:
- use it for budget policy and allowance planning
- monthly changes are less noisy and better reflect stable behavior
- a rising monthly `medianUserSpend` means the typical user is genuinely spending more

How to use these metrics for limits

Daily limits:
- look at `daily...Distribution`
- a high `p99UserSpend` with a low median usually means a few strong outliers
- useful for anti-spike or abuse protection

Weekly limits:
- look at `weekly...Distribution`
- stable weekly `p95UserSpend` can guide soft-limit candidates
- rising weekly `maxUserSpend` may justify a hard cap or alerts

Monthly limits:
- look at `monthly...Distribution`
- this is the best signal for recurring allowance defaults
- if monthly `medianUserSpend` stays low but `p99UserSpend` rises, the tail is getting heavier without broad adoption

Practical recommendation flow

1. Start with monthly series:
- review the trend of `medianUserSpend`, `p95UserSpend`, and `p99UserSpend`
- use this to set or revise monthly allowances

2. Check weekly series:
- see whether usage changes smoothly week to week or has temporary bursts
- if weekly p95 stays stable but max jumps, use alerts before stricter limits

3. Check daily series:
- use it for operational safety and anomaly detection
- daily max and p99 are especially useful for identifying runaway prompts or abuse

Suggested interpretation patterns

Pattern A: Low median, high max, high p99 only on a few days
- normal usage is cheap
- spikes are rare and sharp
- recommended action:
  - keep daily hard caps
  - avoid lowering broad monthly limits unnecessarily

Pattern B: Median and avg both trend upward over weeks and months
- usage is growing across the user base, not just in outliers
- recommended action:
  - revise default weekly/monthly budgets upward if product usage is healthy

Pattern C: Monthly p95 rises while median stays flat
- most users are stable, but heavy-user tail is getting more expensive
- recommended action:
  - keep default limits, but strengthen tail controls for power users

Pattern D: Weekly and monthly max rise together
- expensive usage is not only a single-day anomaly
- recommended action:
  - investigate long-running high-cost users and review policy settings

Limitations of the current model

- these arrays show metric trends over time, not spend histograms by bucket
- percentiles can be noisy when the number of active users in a period is low
- daily values are naturally more volatile than weekly/monthly values
- recommendations should still be combined with product and business context

Suggested UI usage

For each org or course, show:
- a `topFive` table or side panel with:
  - `userId`
  - `spendLastMonth`
- separate charts for:
- daily special
- weekly special
- monthly special
- daily codio-provided
- weekly codio-provided
- monthly codio-provided

For each chart, plot one or more lines:
- `medianUserSpend`
- `p95UserSpend`
- `p99UserSpend`
- optionally `maxUserSpend`

Suggested helper text:
- "Daily p99 shows near-worst-case per-user spend for a single day"
- "Weekly median shows the typical user spend for a full calendar week"
- "Monthly p95 is a strong candidate input for limit policy reviews"

This makes the dashboard useful for monitoring trend shifts, tuning limits, and spotting anomalous growth.
