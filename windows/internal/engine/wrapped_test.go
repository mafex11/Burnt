package engine

import (
	"math"
	"testing"
)

func TestWrappedDataBuildsHeadlineAndModelSplit(t *testing.T) {
	w := NewWrappedData("This Month", 112.40, 47_000_000,
		[]WrappedModel{{"claude-opus-4-8", 90}, {"gpt-5", 22}},
		"Jun 8", 14.2, 0.8, 30.0)

	if w.HeadlineCost != "$112.40" {
		t.Errorf("HeadlineCost = %q, want $112.40", w.HeadlineCost)
	}
	if w.HeadlineTokens != "47.0M" {
		t.Errorf("HeadlineTokens = %q, want 47.0M", w.HeadlineTokens)
	}
	if w.TopModelName != "claude-opus-4-8" {
		t.Errorf("TopModelName = %q, want claude-opus-4-8", w.TopModelName)
	}
	if len(w.ModelBars) != 2 {
		t.Fatalf("ModelBars = %d, want 2", len(w.ModelBars))
	}
	if math.Abs(w.ModelBars[0].Fraction-1.0) > 0.001 {
		t.Errorf("ModelBars[0].Fraction = %v, want 1.0", w.ModelBars[0].Fraction)
	}
	if math.Abs(w.ModelBars[1].Fraction-22.0/90.0) > 0.001 {
		t.Errorf("ModelBars[1].Fraction = %v, want 22/90", w.ModelBars[1].Fraction)
	}
	if w.BusiestDayCost != "$14.20" {
		t.Errorf("BusiestDayCost = %q, want $14.20", w.BusiestDayCost)
	}
	if w.CacheSaved != "$30.00" {
		t.Errorf("CacheSaved = %q, want $30.00", w.CacheSaved)
	}
}

func TestWrappedDataSortsAndCapsAtFiveBars(t *testing.T) {
	models := []WrappedModel{
		{"e", 1}, {"a", 9}, {"c", 5}, {"f", 0.5}, {"b", 7}, {"d", 3},
	}
	w := NewWrappedData("All-Time", 25.5, 100, models, "Jun 8", 9, 1, 0)
	if len(w.ModelBars) != 5 {
		t.Fatalf("ModelBars = %d, want 5", len(w.ModelBars))
	}
	want := []string{"a", "b", "c", "d", "e"}
	for i, name := range want {
		if w.ModelBars[i].Name != name {
			t.Errorf("ModelBars[%d].Name = %q, want %q", i, w.ModelBars[i].Name, name)
		}
	}
}

func TestWrappedDataEmptyModels(t *testing.T) {
	w := NewWrappedData("This Month", 0, 0, nil, "—", 0, 0, 0)
	if w.TopModelName != "—" {
		t.Errorf("TopModelName = %q, want an em dash", w.TopModelName)
	}
	if len(w.ModelBars) != 0 {
		t.Errorf("ModelBars = %+v, want none", w.ModelBars)
	}
}

// An all-zero model list must not divide by zero.
func TestWrappedDataZeroCostModels(t *testing.T) {
	w := NewWrappedData("This Month", 0, 0, []WrappedModel{{"a", 0}}, "—", 0, 0, 0)
	if math.IsNaN(w.ModelBars[0].Fraction) || math.IsInf(w.ModelBars[0].Fraction, 0) {
		t.Errorf("Fraction = %v, want a finite number", w.ModelBars[0].Fraction)
	}
}

func TestBuildWrappedBusiestDayPrefersOldestOnTie(t *testing.T) {
	heat := []DayPoint{{"2026-06-06", 4}, {"2026-06-07", 4}, {"2026-06-08", 1}}
	w := buildWrapped(Totals{}, Totals{}, nil, nil, heat, 0)
	if w.BusiestDay.Date != "2026-06-06" {
		t.Errorf("BusiestDay = %+v, want the older of the tied days", w.BusiestDay)
	}
}

func TestBuildWrappedClaudeShareZeroWhenNoToolCost(t *testing.T) {
	w := buildWrapped(Totals{}, Totals{}, nil, nil, []DayPoint{{"2026-06-08", 0}}, 0)
	if w.ClaudeShare != 0 {
		t.Errorf("ClaudeShare = %v, want 0", w.ClaudeShare)
	}
}
