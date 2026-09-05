// Package engine is a portable port of Burnt's Swift UsageEngine: it locates and
// runs ccusage, decodes its JSON, and aggregates it into a single Summary that the
// UI layer consumes. Everything here is platform-independent except the process
// attributes in runner_windows.go, so the whole package is testable on any OS.
package engine

import (
	"encoding/json"
	"fmt"
)

// Report is `ccusage daily --json`.
type Report struct {
	Daily  []DailyUsage `json:"daily"`
	Totals ReportTotals `json:"totals"`
}

// ReportTotals is the all-time roll-up ccusage reports alongside the daily rows.
type ReportTotals struct {
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

// DailyUsage is one calendar day of usage.
type DailyUsage struct {
	Period              string           `json:"period"` // "2026-06-01", local date
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
	Metadata            *DailyMetadata   `json:"metadata,omitempty"`
}

// DailyMetadata carries which agents contributed to a day.
type DailyMetadata struct {
	Agents []string `json:"agents,omitempty"`
}

// dailyUsageWire mirrors DailyUsage but with both spellings of the day field, so
// UnmarshalJSON can pick whichever ccusage emitted without recursing.
type dailyUsageWire struct {
	Period              *string          `json:"period"`
	Date                *string          `json:"date"`
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
	Metadata            *DailyMetadata   `json:"metadata"`
}

// UnmarshalJSON accepts either "period" (newer ccusage) or "date" (older, e.g.
// 17.1.3) for the day, so a version bump in either direction never silently
// breaks decoding.
func (d *DailyUsage) UnmarshalJSON(data []byte) error {
	var w dailyUsageWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	switch {
	case w.Period != nil:
		d.Period = *w.Period
	case w.Date != nil:
		d.Period = *w.Date
	default:
		return fmt.Errorf("engine: daily row has neither \"period\" nor \"date\"")
	}
	d.InputTokens = w.InputTokens
	d.OutputTokens = w.OutputTokens
	d.CacheCreationTokens = w.CacheCreationTokens
	d.CacheReadTokens = w.CacheReadTokens
	d.TotalTokens = w.TotalTokens
	d.TotalCost = w.TotalCost
	d.ModelBreakdowns = w.ModelBreakdowns
	d.Metadata = w.Metadata
	return nil
}

// ModelBreakdown is one model's slice of a day.
type ModelBreakdown struct {
	ModelName           string  `json:"modelName"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	Cost                float64 `json:"cost"`
}

// SessionReport is `ccusage session --json`.
type SessionReport struct {
	Session []SessionRow `json:"session"`
}

// SessionRow is one session's cost. Period holds the session UUID.
type SessionRow struct {
	Agent       string  `json:"agent,omitempty"`
	Period      string  `json:"period"`
	TotalCost   float64 `json:"totalCost"`
	TotalTokens int64   `json:"totalTokens"`
}

// UnmarshalJSON accepts "period" or "date" for the session id, matching the
// tolerance of the daily decoder.
func (s *SessionRow) UnmarshalJSON(data []byte) error {
	var w struct {
		Agent       string  `json:"agent"`
		Period      *string `json:"period"`
		Date        *string `json:"date"`
		TotalCost   float64 `json:"totalCost"`
		TotalTokens int64   `json:"totalTokens"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	switch {
	case w.Period != nil:
		s.Period = *w.Period
	case w.Date != nil:
		s.Period = *w.Date
	default:
		return fmt.Errorf("engine: session row has neither \"period\" nor \"date\"")
	}
	s.Agent = w.Agent
	s.TotalCost = w.TotalCost
	s.TotalTokens = w.TotalTokens
	return nil
}
