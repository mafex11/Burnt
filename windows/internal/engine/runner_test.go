package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// helperEnv switches the test binary into "pretend to be ccusage" mode. Re-invoking
// ourselves keeps these subprocess tests portable: no shell script, so they exercise the
// real exec path on Windows CI too.
const helperEnv = "BURNT_TEST_CCUSAGE_MODE"

// helperRunner builds a Runner that spawns this test binary as a fake ccusage.
func helperRunner(t *testing.T, mode string, timeout time.Duration) *Runner {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locating the test binary: %v", err)
	}
	t.Setenv(helperEnv, mode)
	return &Runner{
		Invocation: Invocation{
			Executable:  exe,
			LeadingArgs: []string{"-test.run=TestFakeCcusageHelper", "--"},
		},
		Timeout: timeout,
	}
}

// TestFakeCcusageHelper is the fake ccusage. It acts only when it sees BOTH the mode env
// var and the subcommand the Runner appends after "--" — the env var alone isn't enough,
// because it's still set in the parent while a Runner test is in flight.
func TestFakeCcusageHelper(t *testing.T) {
	mode := os.Getenv(helperEnv)
	subcommand := helperSubcommand()
	if mode == "" || subcommand == "" {
		t.Skip("not running as the ccusage helper")
	}
	switch mode {
	case "ok":
		if subcommand == "session" {
			fmt.Print(`{"session":[{"period":"s1","totalCost":1.5,"totalTokens":10}]}`)
		} else {
			fmt.Printf(`{"daily":[{"period":"2026-06-08","inputTokens":1,"outputTokens":2,` +
				`"cacheCreationTokens":3,"cacheReadTokens":4,"totalTokens":10,"totalCost":1.25,` +
				`"modelBreakdowns":[{"modelName":"claude-opus-4-8","inputTokens":1,"outputTokens":2,` +
				`"cacheCreationTokens":3,"cacheReadTokens":4,"cost":1.25}]}],` +
				`"totals":{"inputTokens":1,"outputTokens":2,"cacheCreationTokens":3,` +
				`"cacheReadTokens":4,"totalTokens":10,"totalCost":1.25}}`)
		}
	case "big":
		// Well past a pipe buffer, so a runner that waited for exit before reading would
		// deadlock here.
		filler := strings.Repeat("x", 400_000)
		fmt.Printf(`{"daily":[{"period":"2026-06-08","totalCost":1,"modelBreakdowns":[],`+
			`"note":"%s"}],"totals":{"totalCost":1}}`, filler)
	case "fail":
		fmt.Fprint(os.Stderr, "ccusage: something broke")
		os.Exit(3)
	case "garbage":
		fmt.Print("not json at all")
	case "sleep":
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}

// helperSubcommand is the arg the Runner appended after "--", or "" when we're just an
// ordinary test.
func helperSubcommand() string {
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func TestRunnerFetchDaily(t *testing.T) {
	r := helperRunner(t, "ok", 10*time.Second)
	report, err := r.FetchDaily(context.Background())
	if err != nil {
		t.Fatalf("FetchDaily: %v", err)
	}
	if len(report.Daily) != 1 || report.Daily[0].Period != "2026-06-08" {
		t.Fatalf("Daily = %+v", report.Daily)
	}
	if report.Totals.TotalCost != 1.25 {
		t.Errorf("Totals.TotalCost = %v, want 1.25", report.Totals.TotalCost)
	}
}

func TestRunnerFetchSessions(t *testing.T) {
	r := helperRunner(t, "ok", 10*time.Second)
	report, err := r.FetchSessions(context.Background())
	if err != nil {
		t.Fatalf("FetchSessions: %v", err)
	}
	if len(report.Session) != 1 || report.Session[0].Period != "s1" {
		t.Errorf("Session = %+v", report.Session)
	}
}

// Regression: ccusage emits 100+ KB of JSON, so both pipes must be drained while the
// child runs rather than after it exits.
func TestRunnerDrainsLargeOutput(t *testing.T) {
	r := helperRunner(t, "big", 20*time.Second)
	report, err := r.FetchDaily(context.Background())
	if err != nil {
		t.Fatalf("FetchDaily: %v", err)
	}
	if len(report.Daily) != 1 {
		t.Errorf("Daily = %+v, want the large payload decoded", report.Daily)
	}
}

func TestRunnerNonZeroExitCarriesStderr(t *testing.T) {
	r := helperRunner(t, "fail", 10*time.Second)
	_, err := r.FetchDaily(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want *ExitError", err)
	}
	if exitErr.Code != 3 {
		t.Errorf("Code = %d, want 3", exitErr.Code)
	}
	if !strings.Contains(exitErr.Stderr, "something broke") {
		t.Errorf("Stderr = %q, want the child's message", exitErr.Stderr)
	}
}

func TestRunnerDecodeFailure(t *testing.T) {
	r := helperRunner(t, "garbage", 10*time.Second)
	if _, err := r.FetchDaily(context.Background()); err == nil {
		t.Error("want a decode error for non-JSON output")
	}
}

func TestRunnerTimesOut(t *testing.T) {
	r := helperRunner(t, "sleep", 150*time.Millisecond)
	start := time.Now()
	_, err := r.FetchDaily(context.Background())
	if !errors.Is(err, ErrTimedOut) {
		t.Fatalf("error = %v, want ErrTimedOut", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("took %v, want the timeout to cut it short", elapsed)
	}
}

func TestRunnerHonoursCallerCancellation(t *testing.T) {
	r := helperRunner(t, "sleep", 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.FetchDaily(ctx); err == nil {
		t.Error("want an error for an already-cancelled context")
	}
}

func TestNewRunnerUsesDefaultTimeout(t *testing.T) {
	r := NewRunner(Invocation{Executable: "ccusage"})
	if r.Timeout != DefaultRunTimeout {
		t.Errorf("Timeout = %v, want %v", r.Timeout, DefaultRunTimeout)
	}
}

// Runner must satisfy the interface the Engine consumes.
var _ RunnerIface = (*Runner)(nil)
