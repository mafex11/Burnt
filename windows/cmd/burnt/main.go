// Command burnt is Burnt for Windows: a system tray icon showing what today's Claude
// Code and Codex usage cost, plus a popover dashboard.
//
// Run with no arguments to start the tray app. The three flags exist so the binary can
// be exercised without a desktop session — CI on windows-latest runs all of them, and
// they are the only automated coverage the Windows-only code gets:
//
//	burnt --version                     print "burnt <version>"
//	burnt --render-icon "$12" out.ico   write the tray icon for a label
//	burnt --diagnose out.txt            write a ccusage health report
//
// Because the release build is linked with -H windowsgui it has no console, so
// --diagnose writes to a file rather than stdout.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mafex11/Burnt/windows/internal/app"
	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/trayicon"
)

// version is stamped at build time with -ldflags "-X main.version=1.3.0".
var version = "dev"

const usage = `burnt — Claude Code and Codex spend in your Windows system tray

Usage:
  burnt                            run the tray app
  burnt --version                  print the version
  burnt --render-icon <text> <out> write the tray icon for <text> as an .ico
  burnt --diagnose <out>           write a ccusage health report
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return app.Run(version)
	}

	switch args[0] {
	case "--version", "-version", "-v":
		fmt.Printf("burnt %s\n", version)
		return 0

	case "--render-icon":
		if len(args) != 3 {
			fmt.Fprint(os.Stderr, "usage: burnt --render-icon <text> <out.ico>\n")
			return 2
		}
		if err := renderIcon(args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "burnt: %v\n", err)
			return 1
		}
		return 0

	case "--diagnose":
		if len(args) != 2 {
			fmt.Fprint(os.Stderr, "usage: burnt --diagnose <out.txt>\n")
			return 2
		}
		return diagnose(args[1])

	case "--help", "-h":
		fmt.Print(usage)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "burnt: unknown argument %q\n\n%s", args[0], usage)
		return 2
	}
}

// renderIcon writes the multi-size ICO the tray would show for text. Rendered in the
// dark-taskbar colours, which is what CI eyeballs.
func renderIcon(text, out string) error {
	data, err := trayicon.RenderText(text, false)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(out, data, 0o644)
}

// diagnose writes a report about this install's ccusage and exits non-zero when the
// engine could not produce data. Note that zero daily rows is a healthy answer (a
// fresh machine has no usage yet) — only a missing binary or a failed run is a problem.
func diagnose(out string) int {
	var b strings.Builder
	ok := true

	fmt.Fprintf(&b, "burnt diagnose\n")
	fmt.Fprintf(&b, "time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "version: %s\n", version)
	fmt.Fprintf(&b, "goos/goarch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	exe, err := os.Executable()
	if err != nil {
		exe = fmt.Sprintf("unknown (%v)", err)
	}
	fmt.Fprintf(&b, "exe: %s\n", exe)
	fmt.Fprintf(&b, "ccusage pinned: %s\n", engine.PinnedCcusageVersion)

	inv, err := engine.Locator{}.Resolve()
	if err != nil {
		fmt.Fprintf(&b, "ccusage: not found (%v)\n", err)
		fmt.Fprintf(&b, "hint: %s must sit next to burnt.exe\n", engine.CcusageBinaryName())
		ok = false
	} else {
		fmt.Fprintf(&b, "ccusage: found %s\n", inv.Executable)
		fmt.Fprintf(&b, "ccusage --version: %s\n", ccusageVersion(inv))

		ctx, cancel := context.WithTimeout(context.Background(), engine.DefaultRunTimeout)
		defer cancel()
		report, err := engine.NewRunner(inv).FetchDaily(ctx)
		if err != nil {
			fmt.Fprintf(&b, "daily --json: error: %v\n", err)
			ok = false
		} else {
			fmt.Fprintf(&b, "daily --json: ok (%d rows, all-time $%.2f)\n",
				len(report.Daily), report.Totals.TotalCost)
		}
	}

	fmt.Fprintf(&b, "result: %s\n", map[bool]string{true: "ok", false: "failed"}[ok])

	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "burnt: writing %s: %v\n", out, err)
		return 1
	}
	if !ok {
		return 1
	}
	return 0
}

// ccusageVersion runs `ccusage --version`, reporting the failure inline rather than
// aborting: the version is informational, the daily run below is the real test.
func ccusageVersion(inv engine.Invocation) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := append(append([]string{}, inv.LeadingArgs...), "--version")
	cmd := exec.CommandContext(ctx, inv.Executable, args...)
	app.HideConsole(cmd)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return fmt.Sprintf("error: %v", err)
		}
		return fmt.Sprintf("error: %v (%s)", err, firstLine(text))
	}
	return firstLine(text)
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}
