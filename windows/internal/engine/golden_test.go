package engine

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// goldenPath is the recorded bridge payload. It doubles as the reference shape for
// ui/mock.js, so a diff here is a UI-contract change and deserves a look.
const goldenPath = "testdata/summary-golden.json"

// TestSummaryGolden aggregates the real ccusage fixture at a pinned instant and
// compares the full bridge JSON against a committed golden file. Re-record with
// `go test ./internal/engine -update-golden` after an intentional contract change.
func TestSummaryGolden(t *testing.T) {
	report := decodeReport(t, "daily-normal.json")
	var sessions SessionReport
	if err := json.Unmarshal(fixture(t, "session-sample.json"), &sessions); err != nil {
		t.Fatalf("decoding session-sample.json: %v", err)
	}
	projects := map[string]string{
		"01467451-f660-4bd0-a16c-3298b534e6fd": "/Users/me/code/burnt",
		// The codex session is deliberately unmapped, so the Unknown bucket is covered.
	}

	// 2026-05-05 15:30 UTC: the fixture's newest day, mid-afternoon so projectedToday
	// is populated rather than nil.
	now := time.Date(2026, 5, 5, 15, 30, 0, 0, time.UTC)
	summary := Aggregate(report, &sessions, projects, now, time.UTC)

	data, err := summary.JSON()
	if err != nil {
		t.Fatalf("Summary.JSON: %v", err)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		t.Fatalf("indenting: %v", err)
	}
	got := append(pretty.Bytes(), '\n')

	want, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) || *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Summary JSON differs from %s.\n--- got ---\n%s\n--- want ---\n%s",
			goldenPath, got, want)
	}
}

// TestSummaryGoldenIsDeterministic guards the map-iteration hazards: byTool, byModel and
// byProject all come out of Go maps, so the sorts must impose a total order.
func TestSummaryGoldenIsDeterministic(t *testing.T) {
	report := decodeReport(t, "daily-normal.json")
	var sessions SessionReport
	if err := json.Unmarshal(fixture(t, "session-sample.json"), &sessions); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 5, 15, 30, 0, 0, time.UTC)

	first, err := func() ([]byte, error) {
		s := Aggregate(report, &sessions, nil, now, time.UTC)
		return s.JSON()
	}()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		s := Aggregate(report, &sessions, nil, now, time.UTC)
		again, err := s.JSON()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("Aggregate is not deterministic on run %d", i)
		}
	}
}
