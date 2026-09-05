package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// MaxLogBytes is the size at which burnt.log is thrown away on startup. Burnt runs
// for weeks at a time and nobody rotates its log, so a hard cap checked once per
// launch is all the housekeeping it gets.
const MaxLogBytes int64 = 1 << 20

// openLogFile opens path for appending, creating its directory, and truncates the
// file first when it already exceeds maxBytes.
func openLogFile(path string, maxBytes int64) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("app: create log dir: %w", err)
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > maxBytes {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, fmt.Errorf("app: open log: %w", err)
	}
	return f, nil
}
