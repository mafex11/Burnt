package engine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	sparklineDays = 14 // 14-day sparkline series
	heatmapDays   = 84 // 12 weeks of heatmap cells
	weekDays      = 7  // rolling week window
)

// Aggregate turns a ccusage report into the Summary the UI renders. Pure and
// deterministic: everything time-dependent flows from now and loc, so tests can pin
// both. sessions may be nil (skip project attribution); projects maps session id to
// cwd (see AttributeProjects).
func Aggregate(report Report, sessions *SessionReport, projects map[string]string,
	now time.Time, loc *time.Location) Summary {
	if loc == nil {
		loc = time.Local
	}
	local := now.In(loc)
	// All day arithmetic happens on civil dates pinned to UTC, so a DST transition in
	// loc can never shorten or lengthen a "day" offset.
	today := civilDay(local)
	todayKey := dayKey(today)
	weekStart := today.AddDate(0, 0, -(weekDays - 1))

	var todayTotals Totals
	todayFound := false
	costByDay := make(map[string]float64, len(report.Daily))
	var weekTotals, monthTotals, lastWeekTotals Totals
	// Last week is the 7 days before this week: offsets -13..-7 inclusive.
	lastWeekStart := today.AddDate(0, 0, -13)
	lastWeekEnd := today.AddDate(0, 0, -7)

	toolAgg := map[Tool]*ToolSlice{}
	modelAgg := map[string]*ModelSlice{}
	cacheSavings := 0.0

	for i := range report.Daily {
		d := &report.Daily[i]
		t := dailyTotals(d)
		costByDay[d.Period] += d.TotalCost
		if d.Period == todayKey && !todayFound {
			todayTotals = t
			todayFound = true
		}
		date, ok := parseDay(d.Period)
		if !ok {
			continue
		}
		if date.Year() == today.Year() && date.Month() == today.Month() {
			monthTotals = monthTotals.add(t)
		}
		if !date.Before(lastWeekStart) && !date.After(lastWeekEnd) {
			lastWeekTotals = lastWeekTotals.add(t)
		}
		if date.Before(weekStart) || date.After(today) {
			continue
		}
		weekTotals = weekTotals.add(t)
		for _, m := range d.ModelBreakdowns {
			tool := ClassifyModel(m.ModelName)
			tokens := m.InputTokens + m.OutputTokens + m.CacheCreationTokens + m.CacheReadTokens
			ts, ok := toolAgg[tool]
			if !ok {
				ts = &ToolSlice{Tool: tool}
				toolAgg[tool] = ts
			}
			ts.Cost += m.Cost
			ts.Tokens += tokens
			ms, ok := modelAgg[m.ModelName]
			if !ok {
				ms = &ModelSlice{Model: m.ModelName, Tool: tool}
				modelAgg[m.ModelName] = ms
			}
			ms.Cost += m.Cost
			ms.Tokens += tokens
			cacheSavings += cacheSavingsForModel(m.CacheReadTokens, m.ModelName)
		}
	}

	byTool := make([]ToolSlice, 0, len(toolAgg))
	for _, t := range toolAgg {
		byTool = append(byTool, *t)
	}
	// Cost desc, name asc as the tiebreak — Go map order is random, so an explicit
	// total order is what makes the output reproducible.
	sort.Slice(byTool, func(i, j int) bool {
		if byTool[i].Cost != byTool[j].Cost {
			return byTool[i].Cost > byTool[j].Cost
		}
		return byTool[i].Tool < byTool[j].Tool
	})

	byModel := make([]ModelSlice, 0, len(modelAgg))
	for _, m := range modelAgg {
		byModel = append(byModel, *m)
	}
	sort.Slice(byModel, func(i, j int) bool {
		if byModel[i].Cost != byModel[j].Cost {
			return byModel[i].Cost > byModel[j].Cost
		}
		return byModel[i].Model < byModel[j].Model
	})

	allTime := Totals{
		Cost:                report.Totals.TotalCost,
		InputTokens:         report.Totals.InputTokens,
		OutputTokens:        report.Totals.OutputTokens,
		CacheCreationTokens: report.Totals.CacheCreationTokens,
		CacheReadTokens:     report.Totals.CacheReadTokens,
		TotalTokens:         report.Totals.TotalTokens,
	}

	var trend *float64
	if lastWeekTotals.Cost != 0 {
		v := (weekTotals.Cost - lastWeekTotals.Cost) / lastWeekTotals.Cost
		trend = &v
	}

	byProject := []ProjectSlice{}
	if sessions != nil {
		byProject = GroupProjects(sessions.Session, projects)
	}

	sparkline := series(today, costByDay, sparklineDays)
	heatmap := series(today, costByDay, heatmapDays)

	return Summary{
		Status:         StatusOK,
		FetchedAt:      now,
		Today:          todayTotals,
		Week:           weekTotals,
		Month:          monthTotals,
		AllTime:        allTime,
		LastWeek:       lastWeekTotals,
		AvgPerDay:      weekTotals.Cost / float64(weekDays),
		WeekTrend:      trend,
		ProjectedToday: projectedToday(todayTotals.Cost, local, loc),
		Sparkline:      sparkline,
		Heatmap:        heatmap,
		ByTool:         byTool,
		ByModel:        byModel,
		ByProject:      byProject,
		CacheSavings:   cacheSavings,
		Wrapped:        buildWrapped(monthTotals, allTime, byModel, byTool, heatmap, cacheSavings),
	}
}

// series builds a zero-filled cost series of n days ending on today, oldest first.
func series(today time.Time, costByDay map[string]float64, n int) []DayPoint {
	points := make([]DayPoint, 0, n)
	for offset := n - 1; offset >= 0; offset-- {
		k := dayKey(today.AddDate(0, 0, -offset))
		points = append(points, DayPoint{Date: k, Cost: costByDay[k]})
	}
	return points
}

// projectedToday extrapolates today's spend to a full day. Returns nil before 10% of
// the day has elapsed, where the extrapolation is pure noise.
func projectedToday(todayCost float64, local time.Time, loc *time.Location) *float64 {
	startOfDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	fraction := local.Sub(startOfDay).Seconds() / 86_400.0
	if fraction < 0.1 {
		return nil
	}
	v := todayCost / fraction
	return &v
}

// dailyTotals lifts a daily row into a Totals.
func dailyTotals(d *DailyUsage) Totals {
	return Totals{
		Cost:                d.TotalCost,
		InputTokens:         d.InputTokens,
		OutputTokens:        d.OutputTokens,
		CacheCreationTokens: d.CacheCreationTokens,
		CacheReadTokens:     d.CacheReadTokens,
		TotalTokens:         d.TotalTokens,
	}
}

// civilDay is t's calendar date, re-pinned to midnight UTC for safe day arithmetic.
func civilDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// parseDay parses a "YYYY-MM-DD" period into a civil date. ok is false if the string
// isn't three integer components.
func parseDay(period string) (time.Time, bool) {
	parts := strings.Split(period, "-")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return time.Time{}, false
		}
		nums[i] = n
	}
	return time.Date(nums[0], time.Month(nums[1]), nums[2], 0, 0, 0, 0, time.UTC), true
}

// dayKey renders a civil date as "YYYY-MM-DD".
func dayKey(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
}

// monthKey renders a civil date as "YYYY-MM".
func monthKey(t time.Time) string {
	return fmt.Sprintf("%04d-%02d", t.Year(), int(t.Month()))
}
