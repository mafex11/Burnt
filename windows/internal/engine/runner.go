package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// PinnedCcusageVersion is the ccusage version Burnt ships and tests against. Bump it
// deliberately, after verifying the JSON shape hasn't changed.
const PinnedCcusageVersion = "20.0.6"

// DefaultRunTimeout caps a single ccusage invocation.
const DefaultRunTimeout = 30 * time.Second

// ErrTimedOut means ccusage did not finish inside the timeout.
var ErrTimedOut = errors.New("engine: ccusage timed out")

// ExitError is a non-zero ccusage exit, carrying stderr for diagnostics.
type ExitError struct {
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("engine: ccusage exited %d: %s", e.Code, e.Stderr)
}

// RunnerIface is the subset of Runner the Engine needs, so tests can substitute a
// fake without spawning processes.
type RunnerIface interface {
	FetchDaily(ctx context.Context) (Report, error)
	FetchSessions(ctx context.Context) (SessionReport, error)
}

// Runner executes ccusage subcommands and decodes their JSON.
type Runner struct {
	Invocation Invocation
	// Timeout caps each invocation; zero means DefaultRunTimeout.
	Timeout time.Duration
}

// NewRunner builds a Runner with the default timeout.
func NewRunner(inv Invocation) *Runner {
	return &Runner{Invocation: inv, Timeout: DefaultRunTimeout}
}

// FetchDaily runs `ccusage daily --json`.
//
// Pricing is always fetched online: the pinned ccusage's bundled offline price cache
// lags real model prices and produces wildly wrong figures, so offline mode is unused.
func (r *Runner) FetchDaily(ctx context.Context) (Report, error) {
	var report Report
	err := r.run(ctx, "daily", &report)
	return report, err
}

// FetchSessions runs `ccusage session --json`.
func (r *Runner) FetchSessions(ctx context.Context) (SessionReport, error) {
	var report SessionReport
	err := r.run(ctx, "session", &report)
	return report, err
}

// run invokes one subcommand and decodes stdout into out.
func (r *Runner) run(ctx context.Context, subcommand string, out any) error {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultRunTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append(append([]string{}, r.Invocation.LeadingArgs...), subcommand, "--json")
	cmd := exec.CommandContext(ctx, r.Invocation.Executable, args...)
	hideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	// Drain both pipes WHILE the process runs. ccusage can emit 100+ KB of JSON; if we
	// waited for exit before reading, the child would block on a full pipe buffer and
	// we'd deadlock until the timeout fired.
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&outBuf, stdout) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&errBuf, stderr) }()
	wg.Wait()

	waitErr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return ErrTimedOut
	}
	if waitErr != nil {
		code := -1
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			code = ee.ExitCode()
		}
		return &ExitError{Code: code, Stderr: errBuf.String()}
	}
	if err := json.Unmarshal(outBuf.Bytes(), out); err != nil {
		return fmt.Errorf("engine: decoding ccusage %s output: %w", subcommand, err)
	}
	return nil
}
