package engine

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestLocatorPrefersSiblingBinary(t *testing.T) {
	sibling := filepath.Join("C:\\", "Program Files", "Burnt", "ccusage.exe")
	loc := Locator{
		SiblingBinary: func() string { return sibling },
		// Present on PATH too, but the binary we shipped wins.
		LookPath: func(string) (string, error) { return "C:\\npm\\ccusage.cmd", nil },
	}
	inv, err := loc.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !reflect.DeepEqual(inv, Invocation{Executable: sibling}) {
		t.Errorf("Resolve = %+v, want the sibling binary with no leading args", inv)
	}
}

func TestLocatorFallsBackToPath(t *testing.T) {
	loc := Locator{
		SiblingBinary: func() string { return "" },
		LookPath: func(name string) (string, error) {
			if name == "ccusage" {
				return "/usr/local/bin/ccusage", nil
			}
			return "", errors.New("not found")
		},
	}
	inv, err := loc.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if inv.Executable != "/usr/local/bin/ccusage" {
		t.Errorf("Executable = %q, want /usr/local/bin/ccusage", inv.Executable)
	}
	if len(inv.LeadingArgs) != 0 {
		t.Errorf("LeadingArgs = %v, want none", inv.LeadingArgs)
	}
}

// There is deliberately no npx fallback on Windows: the install bundles a native
// ccusage and never assumes Node exists.
func TestLocatorUnavailableWhenNothingFound(t *testing.T) {
	loc := Locator{
		SiblingBinary: func() string { return "" },
		LookPath:      func(string) (string, error) { return "", errors.New("not found") },
	}
	if _, err := loc.Resolve(); !errors.Is(err, ErrCcusageNotFound) {
		t.Errorf("Resolve error = %v, want ErrCcusageNotFound", err)
	}
}

func TestLocatorTreatsEmptyLookPathResultAsMissing(t *testing.T) {
	loc := Locator{
		SiblingBinary: func() string { return "" },
		LookPath:      func(string) (string, error) { return "", nil },
	}
	if _, err := loc.Resolve(); !errors.Is(err, ErrCcusageNotFound) {
		t.Errorf("Resolve error = %v, want ErrCcusageNotFound", err)
	}
}

func TestCcusageBinaryName(t *testing.T) {
	want := "ccusage"
	if runtime.GOOS == "windows" {
		want = "ccusage.exe"
	}
	if got := CcusageBinaryName(); got != want {
		t.Errorf("CcusageBinaryName = %q, want %q", got, want)
	}
}

func TestDefaultSiblingBinaryMissing(t *testing.T) {
	// The test binary lives in a temp dir with no ccusage beside it.
	if got := DefaultSiblingBinary(); got != "" {
		t.Errorf("DefaultSiblingBinary = %q, want empty next to the test binary", got)
	}
}

func TestPinnedCcusageVersion(t *testing.T) {
	if PinnedCcusageVersion != "20.0.6" {
		t.Errorf("PinnedCcusageVersion = %q, want 20.0.6", PinnedCcusageVersion)
	}
}

func TestSiblingBinaryInFindsExecutable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, CcusageBinaryName())
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := siblingBinaryIn(dir); got != p {
		t.Errorf("siblingBinaryIn = %q, want %q", got, p)
	}
}

// A directory named ccusage must not be mistaken for the binary.
func TestSiblingBinaryInIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, CcusageBinaryName()), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := siblingBinaryIn(dir); got != "" {
		t.Errorf("siblingBinaryIn = %q, want empty for a directory", got)
	}
}

func TestSiblingBinaryInSkipsNonExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("every regular file is executable on Windows")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CcusageBinaryName()), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := siblingBinaryIn(dir); got != "" {
		t.Errorf("siblingBinaryIn = %q, want empty for a non-executable file", got)
	}
}
