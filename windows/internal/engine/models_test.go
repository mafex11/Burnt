package engine

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// fixture reads a ccusage fixture copied from the Swift test suite.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

func decodeReport(t *testing.T, name string) Report {
	t.Helper()
	var r Report
	if err := json.Unmarshal(fixture(t, name), &r); err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
	return r
}

func TestDecodesNormalReport(t *testing.T) {
	r := decodeReport(t, "daily-normal.json")
	if len(r.Daily) == 0 {
		t.Fatal("want daily rows")
	}
	day := r.Daily[0]
	if day.Period == "" {
		t.Error("want a period")
	}
	if len(day.ModelBreakdowns) == 0 {
		t.Error("want model breakdowns")
	}
	if r.Totals.TotalCost < 0 {
		t.Errorf("totals.totalCost = %v, want >= 0", r.Totals.TotalCost)
	}
}

func TestDecodesEmptyReport(t *testing.T) {
	r := decodeReport(t, "daily-empty.json")
	if len(r.Daily) != 0 {
		t.Errorf("daily = %d rows, want 0", len(r.Daily))
	}
}

// Regression: ccusage 17.1.3 emits "date" instead of "period". The decoder must accept
// both so a version bump in either direction can't silently break us.
func TestDecodesPinnedVersionWithDateField(t *testing.T) {
	r := decodeReport(t, "daily-pinned-1713.json")
	if len(r.Daily) == 0 {
		t.Fatal("want daily rows")
	}
	if r.Daily[0].Period == "" {
		t.Error("the \"date\" field must map into Period")
	}
	if len(r.Daily[0].ModelBreakdowns) == 0 {
		t.Error("want model breakdowns")
	}
}

func TestDecodeRejectsRowWithoutDay(t *testing.T) {
	var d DailyUsage
	if err := json.Unmarshal([]byte(`{"inputTokens":1}`), &d); err == nil {
		t.Error("want an error when neither period nor date is present")
	}
}

func TestDecodesSessionReport(t *testing.T) {
	var r SessionReport
	if err := json.Unmarshal(fixture(t, "session-sample.json"), &r); err != nil {
		t.Fatalf("decoding session-sample.json: %v", err)
	}
	if len(r.Session) != 2 {
		t.Fatalf("session rows = %d, want 2", len(r.Session))
	}
	if got, want := r.Session[0].Period, "01467451-f660-4bd0-a16c-3298b534e6fd"; got != want {
		t.Errorf("session[0].Period = %q, want %q", got, want)
	}
	if math.Abs(r.Session[0].TotalCost-14.60) > 0.001 {
		t.Errorf("session[0].TotalCost = %v, want 14.60", r.Session[0].TotalCost)
	}
	if got, want := r.Session[1].Agent, "codex"; got != want {
		t.Errorf("session[1].Agent = %q, want %q", got, want)
	}
}
