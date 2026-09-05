package engine

import (
	"encoding/json"
	"time"
)

// Status describes how trustworthy a Summary is.
type Status string

const (
	StatusOK     Status = "ok"     // fresh data
	StatusStale  Status = "stale"  // last-good cache, fresh fetch failed
	StatusNoData Status = "noData" // nothing cached and nothing fetched
	StatusError  Status = "error"  // ccusage missing or unusable
)

// Totals is a token/cost roll-up over some window.
type Totals struct {
	Cost                float64 `json:"cost"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
}

// add returns a+b field-wise.
func (a Totals) add(b Totals) Totals {
	return Totals{
		Cost:                a.Cost + b.Cost,
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
		TotalTokens:         a.TotalTokens + b.TotalTokens,
	}
}

// DayPoint is one day of the sparkline or heatmap series.
type DayPoint struct {
	Date string  `json:"date"` // "2026-06-01"
	Cost float64 `json:"cost"`
}

// ToolSlice is a per-tool roll-up over the week window.
type ToolSlice struct {
	Tool   Tool    `json:"tool"`
	Cost   float64 `json:"cost"`
	Tokens int64   `json:"tokens"`
}

// ModelSlice is a per-model roll-up over the week window.
type ModelSlice struct {
	Model  string  `json:"model"`
	Tool   Tool    `json:"tool"`
	Cost   float64 `json:"cost"`
	Tokens int64   `json:"tokens"`
}

// ProjectSlice is a per-working-directory roll-up. Path is the dedup key (the full
// cwd, empty for the Unknown bucket) and stays out of the bridge JSON.
type ProjectSlice struct {
	Project string  `json:"project"`
	Path    string  `json:"-"`
	Cost    float64 `json:"cost"`
	Tokens  int64   `json:"tokens"`
}

// Summary is the single payload the UI renders. Its JSON shape is the bridge
// contract shared with ui/mock.js, so field tags here are load-bearing.
type Summary struct {
	Status      Status    `json:"status"`
	StaleReason string    `json:"staleReason"`
	FetchedAt   time.Time `json:"fetchedAt"`

	Today    Totals `json:"today"`
	Week     Totals `json:"week"`     // rolling 7 days ending today
	Month    Totals `json:"month"`    // today's calendar month to date
	AllTime  Totals `json:"allTime"`  // from Report.Totals
	LastWeek Totals `json:"lastWeek"` // the 7 days before this week

	AvgPerDay      float64  `json:"avgPerDay"`      // Week.Cost / 7
	WeekTrend      *float64 `json:"weekTrend"`      // (week-lastWeek)/lastWeek; null if lastWeek is 0
	ProjectedToday *float64 `json:"projectedToday"` // today extrapolated; null before 10% of the day

	Sparkline []DayPoint `json:"sparkline"` // 14 entries, oldest first
	Heatmap   []DayPoint `json:"heatmap"`   // 84 entries, oldest first

	ByTool       []ToolSlice    `json:"byTool"`
	ByModel      []ModelSlice   `json:"byModel"`
	ByProject    []ProjectSlice `json:"byProject"`
	CacheSavings float64        `json:"cacheSavings"`

	Wrapped Wrapped `json:"wrapped"`
}

// JSON renders the bridge payload. Nil slices are normalised to `[]` so the JS side
// never has to null-check an array.
func (s *Summary) JSON() ([]byte, error) {
	out := *s
	if out.Sparkline == nil {
		out.Sparkline = []DayPoint{}
	}
	if out.Heatmap == nil {
		out.Heatmap = []DayPoint{}
	}
	if out.ByTool == nil {
		out.ByTool = []ToolSlice{}
	}
	if out.ByModel == nil {
		out.ByModel = []ModelSlice{}
	}
	if out.ByProject == nil {
		out.ByProject = []ProjectSlice{}
	}
	if out.Wrapped.TopModels == nil {
		out.Wrapped.TopModels = []WrappedModel{}
	}
	return json.Marshal(out)
}
