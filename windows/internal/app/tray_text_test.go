package app

import (
	"strings"
	"testing"

	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/settings"
	"github.com/mafex11/Burnt/windows/internal/trayicon"
)

func sample() engine.Summary {
	return engine.Summary{
		Status: engine.StatusOK,
		Today:  engine.Totals{Cost: 12.34, TotalTokens: 340_000},
		Week:   engine.Totals{Cost: 80.12},
	}
}

func TestTrayText(t *testing.T) {
	s := sample()
	tests := []struct {
		mode settings.MenuBarMode
		want string
	}{
		{settings.ModeTodayCost, trayicon.CompactCost(12.34)},
		{settings.ModeTodayTokens, trayicon.CompactTokens(340_000)},
		{settings.ModeWeekCost, trayicon.CompactCost(80.12)},
		{settings.ModeIconOnly, ""},
	}
	for _, tc := range tests {
		if got := TrayText(s, tc.mode); got != tc.want {
			t.Errorf("TrayText(%s) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

func TestTrayTextDegraded(t *testing.T) {
	for _, status := range []engine.Status{engine.StatusError, engine.StatusNoData} {
		s := sample()
		s.Status = status
		if got := TrayText(s, settings.ModeTodayCost); got != trayicon.PlaceholderText {
			t.Errorf("%s: got %q, want placeholder", status, got)
		}
	}
	// Stale keeps the last good figure rather than blanking the tray.
	s := sample()
	s.Status = engine.StatusStale
	if got := TrayText(s, settings.ModeTodayCost); got == trayicon.PlaceholderText {
		t.Error("stale should keep showing the cached number")
	}
	// Icon-only wins over a degraded status: there is no text either way.
	s.Status = engine.StatusError
	if got := TrayText(s, settings.ModeIconOnly); got != "" {
		t.Errorf("icon-only + error = %q, want empty", got)
	}
}

func TestTooltip(t *testing.T) {
	if got := Tooltip(sample()); got != "Burnt · $12.34 today · $80.12 this week" {
		t.Errorf("got %q", got)
	}

	stale := sample()
	stale.Status = engine.StatusStale
	if got := Tooltip(stale); !strings.HasSuffix(got, "· stale") {
		t.Errorf("stale tooltip = %q", got)
	}

	missing := sample()
	missing.Status = engine.StatusError
	missing.StaleReason = "ccusage not found"
	if got := Tooltip(missing); got != MissingCcusageTooltip {
		t.Errorf("missing ccusage tooltip = %q", got)
	}

	broken := sample()
	broken.Status = engine.StatusError
	broken.StaleReason = strings.Repeat("boom ", 60)
	got := Tooltip(broken)
	if len([]rune(got)) > maxTooltipRunes {
		t.Errorf("tooltip is %d runes, must fit szTip", len([]rune(got)))
	}

	empty := sample()
	empty.Status = engine.StatusNoData
	if got := Tooltip(empty); !strings.Contains(got, "no usage") {
		t.Errorf("noData tooltip = %q", got)
	}
}

func TestMenuTotals(t *testing.T) {
	today, week := MenuTotals(sample())
	if today != "Today: $12.34" || week != "Week: $80.12" {
		t.Errorf("got %q / %q", today, week)
	}
	degraded := sample()
	degraded.Status = engine.StatusError
	today, week = MenuTotals(degraded)
	if !strings.HasSuffix(today, trayicon.PlaceholderText) || !strings.HasSuffix(week, trayicon.PlaceholderText) {
		t.Errorf("degraded menu = %q / %q", today, week)
	}
}

func TestAnimateFlameOnlyInIconOnly(t *testing.T) {
	s := settings.Default()
	s.AnimateFlame = true
	s.MenuBarMode = settings.ModeIconOnly
	if !AnimateFlame(s) {
		t.Error("icon-only + animateFlame should animate")
	}
	s.MenuBarMode = settings.ModeTodayCost
	if AnimateFlame(s) {
		t.Error("text modes have no flame to animate")
	}
	s.MenuBarMode = settings.ModeIconOnly
	s.AnimateFlame = false
	if AnimateFlame(s) {
		t.Error("animateFlame off should not animate")
	}
}

func TestIncludeProjects(t *testing.T) {
	s := settings.Default()
	s.DashboardStyle = settings.StyleDetailed
	if !IncludeProjects(s, true) {
		t.Error("detailed + visible should include projects")
	}
	if IncludeProjects(s, false) {
		t.Error("hidden popover must not pay for project attribution")
	}
	s.DashboardStyle = settings.StyleStandard
	if IncludeProjects(s, true) {
		t.Error("standard does not show the project list")
	}
}
