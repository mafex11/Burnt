package engine

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrCcusageNotFound means no usable ccusage binary exists on this machine.
var ErrCcusageNotFound = errors.New("engine: ccusage not found")

// Invocation is how to invoke ccusage: an executable plus any args that must precede
// the ccusage subcommand.
type Invocation struct {
	Executable  string
	LeadingArgs []string
}

// Locator finds ccusage. Both lookups are injectable so the resolution order can be
// tested without touching the filesystem.
type Locator struct {
	// SiblingBinary returns the path to a ccusage binary shipped next to the running
	// executable, or "" when there isn't one.
	SiblingBinary func() string
	// LookPath resolves a binary name on PATH, as exec.LookPath does.
	LookPath func(name string) (string, error)
}

// Resolve picks an Invocation. Order: the ccusage we ship next to burnt.exe, then
// ccusage on PATH (developer machines). There is deliberately no npx fallback —
// Windows installs bundle a native ccusage and never assume Node is present.
func (l Locator) Resolve() (Invocation, error) {
	sibling := l.SiblingBinary
	if sibling == nil {
		sibling = DefaultSiblingBinary
	}
	if path := sibling(); path != "" {
		return Invocation{Executable: path}, nil
	}
	lookPath := l.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	// Bare name on purpose: Windows' LookPath already tries every PATHEXT suffix, so
	// this finds ccusage.exe, ccusage.cmd or a bare shim alike.
	if path, err := lookPath("ccusage"); err == nil && path != "" {
		return Invocation{Executable: path}, nil
	}
	return Invocation{}, ErrCcusageNotFound
}

// CcusageBinaryName is the file name of the standalone ccusage binary on this OS.
func CcusageBinaryName() string {
	if runtime.GOOS == "windows" {
		return "ccusage.exe"
	}
	return "ccusage"
}

// DefaultSiblingBinary looks for ccusage in the directory holding the running
// executable — the normal path for an installed Burnt, where the release zip puts
// burnt.exe and ccusage.exe side by side.
func DefaultSiblingBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return siblingBinaryIn(filepath.Dir(exe))
}

// siblingBinaryIn returns dir's ccusage binary, or "" when there isn't a usable one.
func siblingBinaryIn(dir string) string {
	candidate := filepath.Join(dir, CcusageBinaryName())
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return ""
	}
	// On Unix the executable bit matters; on Windows any regular file will run.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return ""
	}
	return candidate
}
