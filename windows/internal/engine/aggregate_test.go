package engine

import (
	"math"
	"strings"
	"testing"
	"time"
)

// refDay is the pinned reference date for aggregation tests: 2026-06-08 00:00 UTC.
var refDay = time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)

// report wraps days with all-zero all-time totals.
func report(days ...DailyUsage) Report {
	return Report{Daily: days}
}

// day builds a daily row with only the fields the aggregator reads.
func day(period string, cost float64, models ...ModelBreakdown) DailyUsage {
	return DailyUsage{Period: period, TotalCost: cost, ModelBreakdowns: models}
}

// mb builds a model breakdown whose token total lands in InputTokens.
func mb(name string, cost float64, tokens int64) ModelBreakdown {
	return ModelBreakdown{ModelName: name, Cost: cost, InputTokens: tokens}
}

// aggregate runs Aggregate with the pinned reference date in UTC.
func aggregate(r Report) Summary {
	return Aggregate(r, nil, nil, refDay, time.UTC)
}

func TestTodayPicksMatchingDateOnly(t *testing.T) {
	s := aggregate(report(
		day("2026-06-08", 5, mb("claude-opus-4-8", 5, 100)),
		day("2026-06-07", 9, mb("claude-opus-4-8", 9, 200)),
	))
	if math.Abs(s.Today.Cost-5) > 0.001 {
		t.Errorf("Today.Cost = %v, want 5", s.Today.Cost)
	}
}

func TestWeekIsRolling7DaysInclusive(t *testing.T) {
	// 2026-06-02 is exactly 6 days before 06-08 so it's in; 06-01 is out.
	s := aggregate(report(
		day("2026-06-08", 1, mb("claude-opus-4-8", 1, 10)),
		day("2026-06-02", 2, mb("claude-opus-4-8", 2, 10)),
		day("2026-06-01", 4, mb("claude-opus-4-8", 4, 10)),
	))
	if math.Abs(s.Week.Cost-3) > 0.001 {
		t.Errorf("Week.Cost = %v, want 3 (1+2, not 4)", s.Week.Cost)
	}
}

func TestByToolSplitsClaudeAndCodex(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 7,
		mb("claude-opus-4-8", 5, 100),
		mb("gpt-5.4", 2, 50),
	)))
	if len(s.ByTool) != 2 {
		t.Fatalf("ByTool = %+v, want 2 slices", s.ByTool)
	}
	// Sorted cost desc, so Claude ($5) leads Codex ($2).
	if s.ByTool[0].Tool != ToolClaude || math.Abs(s.ByTool[0].Cost-5) > 0.001 {
		t.Errorf("ByTool[0] = %+v, want claude at $5", s.ByTool[0])
	}
	if s.ByTool[1].Tool != ToolCodex || math.Abs(s.ByTool[1].Cost-2) > 0.001 {
		t.Errorf("ByTool[1] = %+v, want codex at $2", s.ByTool[1])
	}
	if s.ByTool[0].Tokens != 100 {
		t.Errorf("ByTool[0].Tokens = %d, want 100", s.ByTool[0].Tokens)
	}
}

func TestByModelSortsByCostDesc(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 7,
		mb("claude-haiku-4-5", 1, 10),
		mb("claude-opus-4-8", 5, 100),
		mb("gpt-5.4", 2, 50),
	)))
	want := []string{"claude-opus-4-8", "gpt-5.4", "claude-haiku-4-5"}
	for i, w := range want {
		if s.ByModel[i].Model != w {
			t.Errorf("ByModel[%d] = %q, want %q", i, s.ByModel[i].Model, w)
		}
	}
	if s.ByModel[1].Tool != ToolCodex {
		t.Errorf("ByModel[1].Tool = %q, want codex", s.ByModel[1].Tool)
	}
}

func TestSparklineIsZeroFilledFourteenPoints(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 1, mb("claude-opus-4-8", 1, 10))))
	if len(s.Sparkline) != sparklineDays {
		t.Fatalf("Sparkline = %d points, want 14", len(s.Sparkline))
	}
	if got := s.Sparkline[13].Date; got != "2026-06-08" {
		t.Errorf("newest sparkline date = %q, want 2026-06-08", got)
	}
	if got := s.Sparkline[0].Date; got != "2026-05-26" {
		t.Errorf("oldest sparkline date = %q, want 2026-05-26", got)
	}
	if s.Sparkline[0].Cost != 0 {
		t.Errorf("gap day cost = %v, want 0", s.Sparkline[0].Cost)
	}
}

func TestCacheSavingsAggregatesClaudeOnly(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 1,
		ModelBreakdown{ModelName: "claude-opus-4-8", Cost: 1, CacheReadTokens: 1_000_000},
		ModelBreakdown{ModelName: "gpt-5.4", Cost: 1, CacheReadTokens: 1_000_000},
	)))
	if math.Abs(s.CacheSavings-13.50) > 0.01 {
		t.Errorf("CacheSavings = %v, want 13.50 (Claude only)", s.CacheSavings)
	}
}

func TestMonthToDateSumsCalendarMonth(t *testing.T) {
	s := aggregate(report(
		day("2026-06-08", 3, mb("claude-opus-4-8", 3, 10)),
		day("2026-06-02", 4, mb("claude-opus-4-8", 4, 10)),
		day("2026-05-30", 9, mb("claude-opus-4-8", 9, 10)),
	))
	if math.Abs(s.Month.Cost-7) > 0.001 {
		t.Errorf("Month.Cost = %v, want 7", s.Month.Cost)
	}
}

func TestAllTimeFromReportTotals(t *testing.T) {
	r := Report{Totals: ReportTotals{
		InputTokens: 1, OutputTokens: 2, CacheCreationTokens: 3, CacheReadTokens: 4,
		TotalTokens: 10, TotalCost: 99.5,
	}}
	s := aggregate(r)
	if math.Abs(s.AllTime.Cost-99.5) > 0.001 {
		t.Errorf("AllTime.Cost = %v, want 99.5", s.AllTime.Cost)
	}
	if s.AllTime.TotalTokens != 10 {
		t.Errorf("AllTime.TotalTokens = %d, want 10", s.AllTime.TotalTokens)
	}
}

func TestAvgPerDayIsWeekOverSeven(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 7, mb("claude-opus-4-8", 7, 10))))
	if math.Abs(s.AvgPerDay-1.0) > 0.001 {
		t.Errorf("AvgPerDay = %v, want 1.0", s.AvgPerDay)
	}
}

func TestLastWeekWindowAndTrend(t *testing.T) {
	s := aggregate(report(
		day("2026-06-08", 2, mb("claude-opus-4-8", 2, 10)),
		day("2026-06-01", 1, mb("claude-opus-4-8", 1, 10)),
	))
	if math.Abs(s.LastWeek.Cost-1) > 0.001 {
		t.Errorf("LastWeek.Cost = %v, want 1", s.LastWeek.Cost)
	}
	if s.WeekTrend == nil {
		t.Fatal("WeekTrend = nil, want 1.0")
	}
	if math.Abs(*s.WeekTrend-1.0) > 0.001 {
		t.Errorf("WeekTrend = %v, want 1.0", *s.WeekTrend)
	}
}

func TestWeekTrendNilWhenNoLastWeek(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 2, mb("claude-opus-4-8", 2, 10))))
	if s.WeekTrend != nil {
		t.Errorf("WeekTrend = %v, want nil", *s.WeekTrend)
	}
}

func TestProjectedTodayNilEarlyMorning(t *testing.T) {
	early := time.Date(2026, 6, 8, 0, 30, 0, 0, time.UTC)
	s := Aggregate(report(day("2026-06-08", 1, mb("claude-opus-4-8", 1, 10))), nil, nil, early, time.UTC)
	if s.ProjectedToday != nil {
		t.Errorf("ProjectedToday = %v, want nil before 10%% of the day", *s.ProjectedToday)
	}
}

func TestProjectedTodayExtrapolatesMidday(t *testing.T) {
	noon := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	s := Aggregate(report(day("2026-06-08", 5, mb("claude-opus-4-8", 5, 10))), nil, nil, noon, time.UTC)
	if s.ProjectedToday == nil {
		t.Fatal("ProjectedToday = nil, want ~10")
	}
	if math.Abs(*s.ProjectedToday-10.0) > 0.1 {
		t.Errorf("ProjectedToday = %v, want 10", *s.ProjectedToday)
	}
}

func TestHeatmapIsEightyFourZeroFilled(t *testing.T) {
	s := aggregate(report(day("2026-06-08", 1, mb("claude-opus-4-8", 1, 10))))
	if len(s.Heatmap) != heatmapDays {
		t.Fatalf("Heatmap = %d points, want 84", len(s.Heatmap))
	}
	if got := s.Heatmap[83].Date; got != "2026-06-08" {
		t.Errorf("newest heatmap date = %q, want 2026-06-08", got)
	}
	if got := s.Heatmap[0].Date; got != "2026-03-17" {
		t.Errorf("oldest heatmap date = %q, want 2026-03-17", got)
	}
	if math.Abs(s.Heatmap[83].Cost-1) > 0.001 {
		t.Errorf("newest heatmap cost = %v, want 1", s.Heatmap[83].Cost)
	}
}

func TestAggregateUsesLocationForToday(t *testing.T) {
	// 2026-06-08 02:00 UTC is still 2026-06-07 in New York, so "today" must be the 7th.
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	r := report(
		day("2026-06-08", 5, mb("claude-opus-4-8", 5, 10)),
		day("2026-06-07", 9, mb("claude-opus-4-8", 9, 10)),
	)
	s := Aggregate(r, nil, nil, time.Date(2026, 6, 8, 2, 0, 0, 0, time.UTC), ny)
	if math.Abs(s.Today.Cost-9) > 0.001 {
		t.Errorf("Today.Cost = %v, want 9 (local date is 2026-06-07)", s.Today.Cost)
	}
}

func TestAggregateIgnoresUnparseableDates(t *testing.T) {
	s := aggregate(report(
		day("not-a-date", 100, mb("claude-opus-4-8", 100, 10)),
		day("2026-06-08", 1, mb("claude-opus-4-8", 1, 10)),
	))
	if math.Abs(s.Week.Cost-1) > 0.001 {
		t.Errorf("Week.Cost = %v, want 1 (garbage row skipped)", s.Week.Cost)
	}
}

func TestAggregateSumsDuplicateDaysIntoSeries(t *testing.T) {
	// ccusage can emit one row per agent for the same day; the series must add them.
	s := aggregate(report(
		day("2026-06-08", 2, mb("claude-opus-4-8", 2, 10)),
		day("2026-06-08", 3, mb("gpt-5.4", 3, 10)),
	))
	if math.Abs(s.Sparkline[13].Cost-5) > 0.001 {
		t.Errorf("newest sparkline cost = %v, want 5", s.Sparkline[13].Cost)
	}
}

func TestAggregateFillsWrapped(t *testing.T) {
	r := Report{
		Daily: []DailyUsage{
			day("2026-06-08", 3, mb("claude-opus-4-8", 3, 10)),
			day("2026-06-06", 8, mb("gpt-5.4", 8, 10)),
		},
		Totals: ReportTotals{TotalCost: 500},
	}
	s := aggregate(r)
	w := s.Wrapped
	if math.Abs(w.MonthCost-11) > 0.001 {
		t.Errorf("Wrapped.MonthCost = %v, want 11", w.MonthCost)
	}
	if math.Abs(w.AllTimeCost-500) > 0.001 {
		t.Errorf("Wrapped.AllTimeCost = %v, want 500", w.AllTimeCost)
	}
	if len(w.TopModels) != 2 || w.TopModels[0].Model != "gpt-5.4" {
		t.Errorf("Wrapped.TopModels = %+v, want gpt-5.4 first", w.TopModels)
	}
	if w.BusiestDay.Date != "2026-06-06" || math.Abs(w.BusiestDay.Cost-8) > 0.001 {
		t.Errorf("Wrapped.BusiestDay = %+v, want 2026-06-06 at $8", w.BusiestDay)
	}
	if math.Abs(w.ClaudeShare-3.0/11.0) > 0.001 {
		t.Errorf("Wrapped.ClaudeShare = %v, want 3/11", w.ClaudeShare)
	}
}

func TestAggregateWithSessionsFillsByProject(t *testing.T) {
	sessions := &SessionReport{Session: []SessionRow{
		{Period: "a", TotalCost: 5, TotalTokens: 100},
		{Period: "b", TotalCost: 3, TotalTokens: 50},
	}}
	s := Aggregate(report(day("2026-06-08", 8, mb("claude-opus-4-8", 8, 10))), sessions,
		map[string]string{"a": "/Users/me/code/personal", "b": "/Users/me/code/work"}, refDay, time.UTC)
	if len(s.ByProject) != 2 || s.ByProject[0].Project != "personal" {
		t.Errorf("ByProject = %+v, want personal first", s.ByProject)
	}
}

func TestSummaryJSONShape(t *testing.T) {
	var s Summary
	data, err := s.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	// Nil slices must render as [] and the two optional numbers as null.
	for _, want := range []string{
		`"status":""`, `"staleReason":""`, `"fetchedAt":`,
		`"sparkline":[]`, `"heatmap":[]`, `"byTool":[]`, `"byModel":[]`, `"byProject":[]`,
		`"weekTrend":null`, `"projectedToday":null`, `"topModels":[]`,
		`"today":{"cost":0,"inputTokens":0,"outputTokens":0,"cacheCreationTokens":0,"cacheReadTokens":0,"totalTokens":0}`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("Summary JSON missing %s\ngot: %s", want, data)
		}
	}
}
