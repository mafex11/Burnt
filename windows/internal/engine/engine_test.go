package engine

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

// fakeRunner serves canned reports and counts calls, so LoadSummary's caching and
// project-attribution shortcuts are observable. The mutex is for the concurrency test,
// which calls it from several goroutines.
type fakeRunner struct {
	mu         sync.Mutex
	daily      Report
	dailyErr   error
	sessions   SessionReport
	sessionErr error

	dailyCalls   int
	sessionCalls int
}

func (f *fakeRunner) FetchDaily(context.Context) (Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dailyCalls++
	return f.daily, f.dailyErr
}

func (f *fakeRunner) FetchSessions(context.Context) (SessionReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionCalls++
	return f.sessions, f.sessionErr
}

// engineFor builds an Engine pinned to refDay in UTC with no session logs on disk.
func engineFor(r RunnerIface) *Engine {
	return New(r, WithNow(func() time.Time { return refDay }), WithLocation(time.UTC),
		WithProjectDirs("", ""))
}

func TestLoadSummaryReturnsOK(t *testing.T) {
	runner := &fakeRunner{daily: report(day("2026-06-08", 4, mb("claude-opus-4-8", 4, 10)))}
	s := engineFor(runner).LoadSummary(context.Background(), false)
	if s.Status != StatusOK {
		t.Errorf("Status = %q, want ok", s.Status)
	}
	if math.Abs(s.Today.Cost-4) > 0.001 {
		t.Errorf("Today.Cost = %v, want 4", s.Today.Cost)
	}
	if len(s.Sparkline) != sparklineDays || len(s.Heatmap) != heatmapDays {
		t.Errorf("series lengths = %d/%d, want 14/84", len(s.Sparkline), len(s.Heatmap))
	}
	if runner.sessionCalls != 0 {
		t.Errorf("session calls = %d, want 0 when projects are not requested", runner.sessionCalls)
	}
}

func TestLoadSummaryNoDataOnColdFailure(t *testing.T) {
	e := engineFor(&fakeRunner{dailyErr: errors.New("boom")})
	s := e.LoadSummary(context.Background(), false)
	if s.Status != StatusNoData {
		t.Errorf("Status = %q, want noData", s.Status)
	}
	if s.StaleReason != "boom" {
		t.Errorf("StaleReason = %q, want boom", s.StaleReason)
	}
	// The bridge contract needs arrays, not nulls, even in the empty case.
	if s.Sparkline == nil || s.ByModel == nil || s.Wrapped.TopModels == nil {
		t.Error("empty summary must carry empty slices, not nil")
	}
}

func TestLoadSummaryServesStaleCacheOnFailure(t *testing.T) {
	runner := &fakeRunner{daily: report(day("2026-06-08", 4, mb("claude-opus-4-8", 4, 10)))}
	e := engineFor(runner)
	if s := e.LoadSummary(context.Background(), false); s.Status != StatusOK {
		t.Fatalf("first load status = %q, want ok", s.Status)
	}
	runner.dailyErr = errors.New("ccusage exploded")
	s := e.LoadSummary(context.Background(), false)
	if s.Status != StatusStale {
		t.Errorf("Status = %q, want stale", s.Status)
	}
	if s.StaleReason != "ccusage exploded" {
		t.Errorf("StaleReason = %q, want the fetch error", s.StaleReason)
	}
	if math.Abs(s.Today.Cost-4) > 0.001 {
		t.Errorf("Today.Cost = %v, want the cached 4", s.Today.Cost)
	}
}

// An empty report on a cold start is genuinely "no usage".
func TestLoadSummaryEmptyReportColdStartIsNoData(t *testing.T) {
	s := engineFor(&fakeRunner{}).LoadSummary(context.Background(), false)
	if s.Status != StatusNoData {
		t.Errorf("Status = %q, want noData", s.Status)
	}
}

// But once we have good data, a sudden empty result is suspicious: keep the cache as
// stale rather than flashing $0.00.
func TestLoadSummaryEmptyReportAfterGoodDataIsStale(t *testing.T) {
	runner := &fakeRunner{daily: report(day("2026-06-08", 4, mb("claude-opus-4-8", 4, 10)))}
	e := engineFor(runner)
	e.LoadSummary(context.Background(), false)
	runner.daily = Report{}
	s := e.LoadSummary(context.Background(), false)
	if s.Status != StatusStale {
		t.Errorf("Status = %q, want stale", s.Status)
	}
	if s.StaleReason != "ccusage returned no data" {
		t.Errorf("StaleReason = %q", s.StaleReason)
	}
	if math.Abs(s.Today.Cost-4) > 0.001 {
		t.Errorf("Today.Cost = %v, want the cached 4", s.Today.Cost)
	}
}

func TestLoadSummaryWithoutRunnerIsError(t *testing.T) {
	s := New(nil, WithNow(func() time.Time { return refDay })).LoadSummary(context.Background(), false)
	if s.Status != StatusError {
		t.Errorf("Status = %q, want error", s.Status)
	}
	if s.StaleReason == "" {
		t.Error("want a reason explaining ccusage is missing")
	}
}

func TestLoadSummaryIncludesProjectsWhenAsked(t *testing.T) {
	runner := &fakeRunner{
		daily:    report(day("2026-06-08", 8, mb("claude-opus-4-8", 8, 10))),
		sessions: SessionReport{Session: []SessionRow{{Period: "a", TotalCost: 8, TotalTokens: 10}}},
	}
	s := engineFor(runner).LoadSummary(context.Background(), true)
	if runner.sessionCalls != 1 {
		t.Errorf("session calls = %d, want 1", runner.sessionCalls)
	}
	// No session logs on disk, so the one session lands in the Unknown bucket.
	if len(s.ByProject) != 1 || s.ByProject[0].Project != "Unknown" {
		t.Errorf("ByProject = %+v, want a single Unknown bucket", s.ByProject)
	}
}

// A light refresh reuses the previous build's projects so the list doesn't flicker.
func TestLoadSummaryReusesProjectsOnLightRefresh(t *testing.T) {
	runner := &fakeRunner{
		daily:    report(day("2026-06-08", 8, mb("claude-opus-4-8", 8, 10))),
		sessions: SessionReport{Session: []SessionRow{{Period: "a", TotalCost: 8, TotalTokens: 10}}},
	}
	e := engineFor(runner)
	e.LoadSummary(context.Background(), true)
	s := e.LoadSummary(context.Background(), false)
	if len(s.ByProject) != 1 || s.ByProject[0].Project != "Unknown" {
		t.Errorf("ByProject = %+v, want the previous build's list", s.ByProject)
	}
	if runner.sessionCalls != 1 {
		t.Errorf("session calls = %d, want the light refresh to skip sessions", runner.sessionCalls)
	}
}

// A failed session fetch must not sink the whole refresh.
func TestLoadSummarySurvivesSessionFailure(t *testing.T) {
	runner := &fakeRunner{
		daily:      report(day("2026-06-08", 8, mb("claude-opus-4-8", 8, 10))),
		sessionErr: errors.New("session boom"),
	}
	s := engineFor(runner).LoadSummary(context.Background(), true)
	if s.Status != StatusOK {
		t.Errorf("Status = %q, want ok", s.Status)
	}
	if len(s.ByProject) != 0 {
		t.Errorf("ByProject = %+v, want empty", s.ByProject)
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	e := New(nil)
	if e.now == nil || e.loc == nil {
		t.Fatal("New must install a clock and a location")
	}
	if e.claudeDir == "" && e.codexDir == "" {
		t.Skip("no home directory on this machine")
	}
}

func TestLoadSummaryIsSafeForConcurrentCalls(t *testing.T) {
	runner := &fakeRunner{daily: report(day("2026-06-08", 4, mb("claude-opus-4-8", 4, 10)))}
	e := engineFor(runner)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			e.LoadSummary(context.Background(), false)
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}
