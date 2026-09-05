package engine

import (
	"fmt"
	"time"
)

// milestoneLevels are the month-to-date spend thresholds worth telling the user about.
var milestoneLevels = []float64{50, 100, 250, 500, 1000}

// budgetWarnRatio is the fraction of the daily budget that triggers a heads-up.
const budgetWarnRatio = 0.8

// NotifySettings mirrors the user's notification preferences.
type NotifySettings struct {
	BudgetAlerts bool    `json:"notifyBudget"`
	DailySummary bool    `json:"notifyDailySummary"`
	Milestones   bool    `json:"notifyMilestones"`
	DailyBudget  float64 `json:"dailyBudget"`
}

// anyEnabled reports whether any notification category is on.
func (s NotifySettings) anyEnabled() bool {
	return s.BudgetAlerts || s.DailySummary || s.Milestones
}

// NotifyState records which notifications have already fired, so each one shows once
// per day or month. Persist it alongside settings.
type NotifyState struct {
	Fired map[string]bool `json:"fired"`
}

// clone copies the state so EvaluateNotifications stays pure with respect to its input.
func (s NotifyState) clone() NotifyState {
	out := NotifyState{Fired: make(map[string]bool, len(s.Fired)+len(milestoneLevels))}
	for k, v := range s.Fired {
		out.Fired[k] = v
	}
	return out
}

// Notification is one toast to post.
type Notification struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// EvaluateNotifications decides which toasts a Summary warrants. Pure: the "already
// fired" set is injected and the updated set is returned, so the caller owns
// persistence and the rules are directly testable.
func EvaluateNotifications(s Summary, settings NotifySettings, state NotifyState,
	now time.Time) ([]Notification, NotifyState) {
	next := state.clone()
	if !settings.anyEnabled() {
		return nil, next
	}

	day := dayKey(civilDay(now))
	month := monthKey(civilDay(now))
	var out []Notification
	// fireOnce is what makes every rule idempotent: an id already in the set is skipped.
	fireOnce := func(id, title, body string) {
		if next.Fired[id] {
			return
		}
		next.Fired[id] = true
		out = append(out, Notification{ID: id, Title: title, Body: body})
	}

	if settings.BudgetAlerts && settings.DailyBudget > 0 {
		ratio := s.Today.Cost / settings.DailyBudget
		if ratio >= budgetWarnRatio {
			fireOnce("budget80-"+day, "80% of daily budget",
				fmt.Sprintf("Today: %s of %s.", money(s.Today.Cost), money(settings.DailyBudget)))
		}
		if ratio >= 1.0 {
			fireOnce("budget100-"+day, "Daily budget reached",
				fmt.Sprintf("Today: %s — over your %s cap.", money(s.Today.Cost), money(settings.DailyBudget)))
		}
	}

	if settings.Milestones {
		for _, level := range milestoneLevels {
			if s.Month.Cost < level {
				continue
			}
			fireOnce(fmt.Sprintf("milestone%d-%s", int(level), month),
				fmt.Sprintf("Burnt %s this month", money(level)),
				fmt.Sprintf("Month to date: %s.", money(s.Month.Cost)))
		}
	}

	if settings.DailySummary {
		suffix := ""
		if model := topModelName(s); model != "" {
			suffix = " · mostly " + model
		}
		fireOnce("summary-"+day, "Yesterday on Burnt",
			fmt.Sprintf("%s burnt%s.", money(yesterdayCost(s)), suffix))
	}

	return out, next
}

// yesterdayCost is the second-newest heatmap point, i.e. the day before today.
func yesterdayCost(s Summary) float64 {
	if len(s.Heatmap) < 2 {
		return 0
	}
	return s.Heatmap[len(s.Heatmap)-2].Cost
}

// topModelName is the week's costliest model, or "" when there is none.
func topModelName(s Summary) string {
	if len(s.ByModel) == 0 {
		return ""
	}
	return s.ByModel[0].Model
}

// money is the plain notification-body money format (always cents, no grouping).
func money(v float64) string { return fmt.Sprintf("$%.2f", v) }
