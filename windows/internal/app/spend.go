package app

import "github.com/mafex11/Burnt/windows/internal/engine"

// minSpendRise is the smallest rise in today's cost that counts as new spend;
// anything smaller is rounding noise between polls.
const minSpendRise = 0.005

// SpendWatcher decides when the flame should flare: only when today's spend rose
// since the previous poll. The first reading only sets the baseline (no flare at
// launch), and a drop (today's cost resetting at midnight) just lowers it.
type SpendWatcher struct {
	last    float64
	hasLast bool
}

// Observe records today's cost from a fresh summary and reports whether it rose.
// Summaries without usable data (error, no data) are ignored.
func (w *SpendWatcher) Observe(s engine.Summary) bool {
	if s.Status != engine.StatusOK && s.Status != engine.StatusStale {
		return false
	}
	cost := s.Today.Cost
	rose := w.hasLast && cost-w.last >= minSpendRise
	w.last, w.hasLast = cost, true
	return rose
}
