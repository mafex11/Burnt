// Package settings stores Burnt's user preferences as JSON on disk.
//
// The file lives at %APPDATA%\Burnt\settings.json on Windows (os.UserConfigDir
// on other platforms, which keeps the package testable on macOS). Keys and
// defaults mirror the macOS app's UserDefaults so both ports behave the same.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MenuBarMode selects what the tray icon renders.
type MenuBarMode string

const (
	ModeTodayCost   MenuBarMode = "todayCost"
	ModeTodayTokens MenuBarMode = "todayTokens"
	ModeWeekCost    MenuBarMode = "weekCost"
	ModeIconOnly    MenuBarMode = "iconOnly"
)

// Valid reports whether m is one of the four known modes.
func (m MenuBarMode) Valid() bool {
	switch m {
	case ModeTodayCost, ModeTodayTokens, ModeWeekCost, ModeIconOnly:
		return true
	}
	return false
}

// DashboardStyle is the popover's information density.
type DashboardStyle string

const (
	StyleMinimal  DashboardStyle = "minimal"
	StyleStandard DashboardStyle = "standard"
	StyleDetailed DashboardStyle = "detailed"
)

// Valid reports whether s is one of the three known styles.
func (s DashboardStyle) Valid() bool {
	switch s {
	case StyleMinimal, StyleStandard, StyleDetailed:
		return true
	}
	return false
}

// Rank orders styles so callers can gate sections additively
// (minimal < standard < detailed).
func (s DashboardStyle) Rank() int {
	switch s {
	case StyleMinimal:
		return 0
	case StyleDetailed:
		return 2
	default:
		return 1
	}
}

// AtLeast reports whether s shows everything other does, and possibly more.
func (s DashboardStyle) AtLeast(other DashboardStyle) bool {
	return s.Rank() >= other.Rank()
}

// Settings is the whole persisted preference set.
type Settings struct {
	MenuBarMode        MenuBarMode    `json:"menuBarMode"`
	DailyBudget        float64        `json:"dailyBudget"` // 0 = off
	DashboardStyle     DashboardStyle `json:"dashboardStyle"`
	NotifyBudget       bool           `json:"notifyBudget"`
	NotifyDailySummary bool           `json:"notifyDailySummary"`
	NotifyMilestones   bool           `json:"notifyMilestones"`
	AnimateFlame       bool           `json:"animateFlame"`
	AutoUpdate         bool           `json:"autoUpdate"`
	LastUpdateCheck    string         `json:"lastUpdateCheck"` // RFC3339, or "" when never
}

// Default returns the settings a fresh install starts with.
func Default() Settings {
	return Settings{
		MenuBarMode:        ModeTodayCost,
		DailyBudget:        0,
		DashboardStyle:     StyleStandard,
		NotifyBudget:       false,
		NotifyDailySummary: false,
		NotifyMilestones:   false,
		AnimateFlame:       true,
		AutoUpdate:         true,
		LastUpdateCheck:    "",
	}
}

// Validate coerces out-of-range enum values back to their defaults, leaving
// everything else untouched. Always call it on anything read from disk or JS.
func (s Settings) Validate() Settings {
	if !s.MenuBarMode.Valid() {
		s.MenuBarMode = ModeTodayCost
	}
	if !s.DashboardStyle.Valid() {
		s.DashboardStyle = StyleStandard
	}
	if s.DailyBudget < 0 {
		s.DailyBudget = 0
	}
	return s
}

// DefaultPath is %APPDATA%\Burnt\settings.json on Windows, and the equivalent
// user config directory elsewhere.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "Burnt", "settings.json"), nil
}

// Load reads path. A missing file yields defaults with no error; a partial file
// keeps defaults for every key it omits. Malformed JSON is an error so we never
// silently discard a user's preferences.
func Load(path string) (Settings, error) {
	s := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return Default(), fmt.Errorf("read settings %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), fmt.Errorf("parse settings %s: %w", path, err)
	}
	return s.Validate(), nil
}

// Save atomically writes s to path, creating parent directories as needed.
func Save(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	data, err := json.MarshalIndent(s.Validate(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return fmt.Errorf("create temp settings: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp settings: %w", err)
	}
	// Windows rename fails onto an existing file, so clear the target first.
	_ = os.Remove(path)
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}
