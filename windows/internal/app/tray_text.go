package app

import (
	"strings"

	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/settings"
	"github.com/mafex11/Burnt/windows/internal/trayicon"
)

// MissingCcusageTooltip is what the tray says when the engine could not find ccusage.
// That is the one failure a user can actually fix, so it is spelled out in full.
const MissingCcusageTooltip = "ccusage.exe not found next to burnt.exe"

// maxTooltipRunes keeps us inside NOTIFYICONDATA's 128-character szTip.
const maxTooltipRunes = 120

// TrayText is the string to draw into the tray icon.
//
// "" means "draw the flame instead" (icon-only mode). trayicon.PlaceholderText means
// "there is no trustworthy number yet", which the caller renders with
// trayicon.Placeholder(). Anything else is a compact figure to rasterise.
func TrayText(s engine.Summary, mode settings.MenuBarMode) string {
	if mode == settings.ModeIconOnly {
		return ""
	}
	// A stale summary still carries the last good numbers, so it keeps showing them;
	// error and noData have nothing worth putting in 16 px.
	if s.Status == engine.StatusError || s.Status == engine.StatusNoData {
		return trayicon.PlaceholderText
	}
	switch mode {
	case settings.ModeTodayTokens:
		return trayicon.CompactTokens(s.Today.TotalTokens)
	case settings.ModeWeekCost:
		return trayicon.CompactCost(s.Week.Cost)
	default:
		return trayicon.CompactCost(s.Today.Cost)
	}
}

// Tooltip is the hover text: the full-precision figures the icon has no room for.
func Tooltip(s engine.Summary) string {
	var text string
	switch s.Status {
	case engine.StatusError:
		text = MissingCcusageTooltip
		if s.StaleReason != "" && !strings.Contains(s.StaleReason, "not found") {
			text = "Burnt · " + s.StaleReason
		}
	case engine.StatusNoData:
		text = "Burnt · no usage yet"
	default:
		text = "Burnt · " + engine.FormatCost(s.Today.Cost) + " today · " +
			engine.FormatCost(s.Week.Cost) + " this week"
		if s.Status == engine.StatusStale {
			text += " · stale"
		}
	}
	runes := []rune(text)
	if len(runes) > maxTooltipRunes {
		text = string(runes[:maxTooltipRunes-1]) + "…"
	}
	return text
}

// MenuTotals are the two disabled rows in the tray menu.
func MenuTotals(s engine.Summary) (today, week string) {
	if s.Status == engine.StatusError || s.Status == engine.StatusNoData {
		return "Today: " + trayicon.PlaceholderText, "Week: " + trayicon.PlaceholderText
	}
	return "Today: " + engine.FormatCost(s.Today.Cost), "Week: " + engine.FormatCost(s.Week.Cost)
}

// AnimateFlame reports whether the flame ticker should be running: only icon-only
// mode has a flame to animate, so the text modes never pay for the timer.
func AnimateFlame(s settings.Settings) bool {
	return s.AnimateFlame && s.MenuBarMode == settings.ModeIconOnly
}

// IncludeProjects reports whether a refresh should pay for per-project attribution:
// a second subprocess plus a walk of every session log, which only the Detailed
// dashboard displays and only while it is on screen.
func IncludeProjects(s settings.Settings, popoverVisible bool) bool {
	return popoverVisible && s.DashboardStyle == settings.StyleDetailed
}
