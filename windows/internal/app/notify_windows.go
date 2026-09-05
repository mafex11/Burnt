package app

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/settings"
)

// flameFS is Burnt's app icon, used as the toast image. Windows toasts read their
// image from a file path, so it is written to %APPDATA%\Burnt once per launch.
// (This is a copy of winres/icon-256.png; internal/trayicon exports rendered ICOs,
// not the source PNG.)
//
//go:embed flame.png
var flameFS embed.FS

const notifyStateFile = "notify-state.json"

// notifier posts toasts and remembers which ones already fired.
type notifier struct {
	dataDir  string
	iconPath string
}

func newNotifier(dataDir string) *notifier {
	beeep.AppName = "Burnt"
	n := &notifier{dataDir: dataDir}
	if path, err := n.writeIcon(); err != nil {
		log.Printf("notify: no toast icon: %v", err)
	} else {
		n.iconPath = path
	}
	return n
}

// writeIcon materialises the embedded flame next to the settings file.
func (n *notifier) writeIcon() (string, error) {
	data, err := flameFS.ReadFile("flame.png")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(n.dataDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(n.dataDir, "flame.png")
	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(data)) {
		return path, nil // already written by an earlier launch
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Evaluate posts whatever notifications this summary has newly earned and persists
// the fired set, so a budget alert or daily summary shows once and not on every poll.
func (n *notifier) Evaluate(s engine.Summary, cfg settings.Settings, now func() time.Time) {
	if s.Status != engine.StatusOK && s.Status != engine.StatusStale {
		return
	}
	if !cfg.NotifyBudget && !cfg.NotifyDailySummary && !cfg.NotifyMilestones {
		return
	}

	state := n.loadState()
	notes, next := engine.EvaluateNotifications(s, engine.NotifySettings{
		BudgetAlerts: cfg.NotifyBudget,
		DailySummary: cfg.NotifyDailySummary,
		Milestones:   cfg.NotifyMilestones,
		DailyBudget:  cfg.DailyBudget,
	}, state, now())
	if len(notes) == 0 {
		return
	}
	if err := n.saveState(next); err != nil {
		// Posting without persisting would repeat the toast every minute, so bail.
		log.Printf("notify: save state: %v", err)
		return
	}
	for _, note := range notes {
		if err := beeep.Notify(note.Title, note.Body, n.iconPath); err != nil {
			log.Printf("notify: %s: %v", note.ID, err)
		}
	}
}

// Post shows a one-off toast that has nothing to do with usage thresholds.
func (n *notifier) Post(title, body string) {
	if err := beeep.Notify(title, body, n.iconPath); err != nil {
		log.Printf("notify: %s: %v", title, err)
	}
}

func (n *notifier) statePath() string { return filepath.Join(n.dataDir, notifyStateFile) }

func (n *notifier) loadState() engine.NotifyState {
	state := engine.NotifyState{Fired: map[string]bool{}}
	data, err := os.ReadFile(n.statePath())
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Fired == nil {
		return engine.NotifyState{Fired: map[string]bool{}}
	}
	return state
}

func (n *notifier) saveState(state engine.NotifyState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(n.dataDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(n.statePath(), data, 0o644); err != nil {
		return fmt.Errorf("write notify state: %w", err)
	}
	return nil
}
