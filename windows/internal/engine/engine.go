package engine

import (
	"context"
	"sync"
	"time"
)

// Engine turns a Runner into Summaries, remembering the last good one so a transient
// ccusage failure shows slightly stale numbers instead of an empty dashboard.
type Engine struct {
	runner RunnerIface
	now    func() time.Time
	loc    *time.Location

	claudeDir string
	codexDir  string

	mu       sync.Mutex
	lastGood *Summary
}

// Option configures an Engine.
type Option func(*Engine)

// WithNow overrides the clock, for tests.
func WithNow(now func() time.Time) Option {
	return func(e *Engine) {
		if now != nil {
			e.now = now
		}
	}
}

// WithLocation overrides the time zone used for day boundaries.
func WithLocation(loc *time.Location) Option {
	return func(e *Engine) {
		if loc != nil {
			e.loc = loc
		}
	}
}

// WithProjectDirs overrides the Claude and Codex session-log roots.
func WithProjectDirs(claudeDir, codexDir string) Option {
	return func(e *Engine) {
		e.claudeDir, e.codexDir = claudeDir, codexDir
	}
}

// New builds an Engine. A nil runner is legal and yields StatusError summaries, which
// is what "ccusage isn't installed" looks like to the UI.
func New(runner RunnerIface, opts ...Option) *Engine {
	claudeDir, codexDir := DefaultProjectDirs()
	e := &Engine{
		runner:    runner,
		now:       time.Now,
		loc:       time.Local,
		claudeDir: claudeDir,
		codexDir:  codexDir,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// LoadSummary fetches usage with live pricing.
//
// includeProjects controls the expensive per-project attribution: it runs a second
// `ccusage session` subprocess AND walks every Claude/Codex session log on disk. That
// data only feeds the "By project" list, so the tray poll passes false and only pays
// for it when the popover asks. When skipped, the previous build's projects are reused
// so the list doesn't flicker.
func (e *Engine) LoadSummary(ctx context.Context, includeProjects bool) Summary {
	if e.runner == nil {
		return e.degraded(StatusError, "ccusage not found")
	}

	report, err := e.runner.FetchDaily(ctx)
	if err != nil {
		return e.degraded(StatusStale, err.Error())
	}

	// An empty report means no usage. Only trust that on a genuine cold start. If we
	// already have good data, a sudden empty result is suspicious (a transient read
	// issue), so keep the cache as stale rather than flashing $0.00.
	if len(report.Daily) == 0 {
		return e.degraded(StatusStale, "ccusage returned no data")
	}

	var sessions *SessionReport
	projects := map[string]string{}
	if includeProjects {
		if s, err := e.runner.FetchSessions(ctx); err == nil {
			sessions = &s
			projects = AttributeProjects(e.claudeDir, e.codexDir)
		}
	}

	summary := Aggregate(report, sessions, projects, e.now(), e.loc)
	if !includeProjects || sessions == nil {
		summary.ByProject = e.cachedProjects()
	}

	e.mu.Lock()
	cached := summary
	e.lastGood = &cached
	e.mu.Unlock()
	return summary
}

// degraded serves the last good summary re-badged with why the fresh fetch failed, or
// a bare noData summary when there is nothing cached.
func (e *Engine) degraded(status Status, reason string) Summary {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lastGood == nil {
		// Nothing cached: "stale" would be a lie, so a failed fetch degrades to noData
		// while a missing ccusage stays an error the UI can call out.
		empty := StatusNoData
		if status == StatusError {
			empty = StatusError
		}
		return Summary{
			Status:      empty,
			StaleReason: reason,
			FetchedAt:   e.now(),
			Sparkline:   []DayPoint{},
			Heatmap:     []DayPoint{},
			ByTool:      []ToolSlice{},
			ByModel:     []ModelSlice{},
			ByProject:   []ProjectSlice{},
			Wrapped:     Wrapped{TopModels: []WrappedModel{}},
		}
	}
	out := *e.lastGood
	out.Status = status
	out.StaleReason = reason
	return out
}

// cachedProjects is the previous build's project list, so a light (no-project) refresh
// keeps "By project" populated rather than briefly emptying it.
func (e *Engine) cachedProjects() []ProjectSlice {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lastGood == nil {
		return []ProjectSlice{}
	}
	return e.lastGood.ByProject
}
