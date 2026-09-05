//go:build !windows

package update

import (
	"errors"
	"fmt"
	"runtime"
)

// ErrUnsupported is returned by Apply on platforms other than Windows.
var ErrUnsupported = errors.New("update: in-place apply is only supported on Windows")

// Apply is not available off Windows; the macOS build updates via Homebrew.
func Apply(zipPath, installDir, exePath string) error {
	return fmt.Errorf("%w (running on %s)", ErrUnsupported, runtime.GOOS)
}
