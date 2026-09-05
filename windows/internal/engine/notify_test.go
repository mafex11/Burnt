package engine

import (
	"strings"
	"testing"
	"time"
)

var notifyNow = time.Date(2026, 6, 8, 15, 0, 0, 0, time.UTC)

// notifySummary builds the minimal Summary the notification rules read.
func notifySummary(today, month, yesterday float64, topModel string) Summary {
	s := Summary{
		Today:   Totals{Cost: today},
		Month:   Totals{Cost: month},
		Heatmap: []DayPoint{{"2026-06-07", yesterday}, {"2026-06-08", today}},
	}
	if topModel != "" {
		s.ByModel = []ModelSlice{{Model: topModel, Tool: ToolClaude, Cost: today}}
	}
	return s
}

// hasID reports whether any notification's id contains sub.
func hasID(ns []Notification, sub string) bool {
	for _, n := range ns {
		if strings.Contains(n.ID, sub) {
			return true
		}
	}
	return false
}

func TestBudget80And100FireOncePerDay(t *testing.T) {
	settings := NotifySettings{BudgetAlerts: true, DailyBudget: 10}
	state := NotifyState{}

	out, state := EvaluateNotifications(notifySummary(9, 9, 0, ""), settings, state, notifyNow)
	if !hasID(out, "budget80") {
		t.Errorf("want budget80, got %+v", out)
	}
	if hasID(out, "budget100") {
		t.Errorf("budget100 fired too early: %+v", out)
	}

	out, state = EvaluateNotifications(notifySummary(9, 9, 0, ""), settings, state, notifyNow)
	if len(out) != 0 {
		t.Errorf("want nothing on the second evaluation, got %+v", out)
	}

	out, _ = EvaluateNotifications(notifySummary(11, 11, 0, ""), settings, state, notifyNow)
	if !hasID(out, "budget100") {
		t.Errorf("want budget100 once over the cap, got %+v", out)
	}
	if hasID(out, "budget80") {
		t.Errorf("budget80 fired twice: %+v", out)
	}
}

func TestBudgetKeysAreScopedToTheDay(t *testing.T) {
	settings := NotifySettings{BudgetAlerts: true, DailyBudget: 10}
	_, state := EvaluateNotifications(notifySummary(9, 9, 0, ""), settings, NotifyState{}, notifyNow)
	tomorrow := notifyNow.AddDate(0, 0, 1)
	out, _ := EvaluateNotifications(notifySummary(9, 9, 0, ""), settings, state, tomorrow)
	if !hasID(out, "budget80-2026-06-09") {
		t.Errorf("a new day must re-arm the budget alert, got %+v", out)
	}
}

func TestNoBudgetAlertsWithoutABudget(t *testing.T) {
	settings := NotifySettings{BudgetAlerts: true, DailyBudget: 0}
	out, _ := EvaluateNotifications(notifySummary(99, 99, 0, ""), settings, NotifyState{}, notifyNow)
	if len(out) != 0 {
		t.Errorf("want nothing without a budget, got %+v", out)
	}
}

func TestNoNotificationsWhenAllDisabled(t *testing.T) {
	out, state := EvaluateNotifications(notifySummary(99, 999, 12, "opus"), NotifySettings{DailyBudget: 10},
		NotifyState{}, notifyNow)
	if len(out) != 0 {
		t.Errorf("want nothing when every category is off, got %+v", out)
	}
	if len(state.Fired) != 0 {
		t.Errorf("state must stay clean, got %v", state.Fired)
	}
}

func TestMilestonesFireOncePerMonth(t *testing.T) {
	settings := NotifySettings{Milestones: true}
	out, state := EvaluateNotifications(notifySummary(1, 120, 0, ""), settings, NotifyState{}, notifyNow)
	if !hasID(out, "milestone100") {
		t.Errorf("want milestone100, got %+v", out)
	}
	if !hasID(out, "milestone50") {
		t.Errorf("want the crossed milestone50 too, got %+v", out)
	}
	if hasID(out, "milestone250") {
		t.Errorf("milestone250 fired below its level: %+v", out)
	}

	out, state = EvaluateNotifications(notifySummary(1, 130, 0, ""), settings, state, notifyNow)
	if len(out) != 0 {
		t.Errorf("want nothing on re-evaluation, got %+v", out)
	}

	out, _ = EvaluateNotifications(notifySummary(1, 300, 0, ""), settings, state, notifyNow)
	if !hasID(out, "milestone250") {
		t.Errorf("want milestone250 once crossed, got %+v", out)
	}
}

func TestMilestoneKeysAreScopedToTheMonth(t *testing.T) {
	settings := NotifySettings{Milestones: true}
	_, state := EvaluateNotifications(notifySummary(1, 120, 0, ""), settings, NotifyState{}, notifyNow)
	nextMonth := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	out, _ := EvaluateNotifications(notifySummary(1, 120, 0, ""), settings, state, nextMonth)
	if !hasID(out, "milestone100-2026-07") {
		t.Errorf("a new month must re-arm milestones, got %+v", out)
	}
}

func TestDailySummaryFiresOncePerDay(t *testing.T) {
	settings := NotifySettings{DailySummary: true}
	out, state := EvaluateNotifications(notifySummary(1, 1, 12.4, "opus"), settings, NotifyState{}, notifyNow)
	if !hasID(out, "summary") {
		t.Fatalf("want a daily summary, got %+v", out)
	}
	if got := out[0].Body; got != "$12.40 burnt · mostly opus." {
		t.Errorf("body = %q, want yesterday's cost and top model", got)
	}

	out, _ = EvaluateNotifications(notifySummary(1, 1, 12.4, "opus"), settings, state, notifyNow)
	if len(out) != 0 {
		t.Errorf("want nothing on the second evaluation, got %+v", out)
	}
}

func TestDailySummaryOmitsModelWhenUnknown(t *testing.T) {
	settings := NotifySettings{DailySummary: true}
	out, _ := EvaluateNotifications(notifySummary(1, 1, 3, ""), settings, NotifyState{}, notifyNow)
	if got := out[0].Body; got != "$3.00 burnt." {
		t.Errorf("body = %q, want no model clause", got)
	}
}

// EvaluateNotifications must not mutate the caller's state map.
func TestEvaluateNotificationsDoesNotMutateInputState(t *testing.T) {
	settings := NotifySettings{BudgetAlerts: true, DailyBudget: 10}
	state := NotifyState{Fired: map[string]bool{}}
	_, next := EvaluateNotifications(notifySummary(20, 20, 0, ""), settings, state, notifyNow)
	if len(state.Fired) != 0 {
		t.Errorf("input state was mutated: %v", state.Fired)
	}
	if len(next.Fired) != 2 {
		t.Errorf("returned state = %v, want budget80 and budget100 recorded", next.Fired)
	}
}

func TestNotifyBodiesUsePlainMoney(t *testing.T) {
	settings := NotifySettings{BudgetAlerts: true, DailyBudget: 2000}
	out, _ := EvaluateNotifications(notifySummary(2500, 2500, 0, ""), settings, NotifyState{}, notifyNow)
	if !strings.Contains(out[0].Body, "$2500.00") {
		t.Errorf("body = %q, want ungrouped cents in notification copy", out[0].Body)
	}
}
