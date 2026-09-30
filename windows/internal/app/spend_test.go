package app

import (
	"testing"

	"github.com/mafex11/Burnt/windows/internal/engine"
)

func today(cost float64) engine.Summary {
	return engine.Summary{Status: engine.StatusOK, Today: engine.Totals{Cost: cost}}
}

func TestSpendWatcherFirstReadingOnlySetsBaseline(t *testing.T) {
	var w SpendWatcher
	if w.Observe(today(12.40)) {
		t.Error("first reading flared, want baseline only")
	}
}

func TestSpendWatcherRiseFlares(t *testing.T) {
	var w SpendWatcher
	w.Observe(today(12.40))
	if !w.Observe(today(12.55)) {
		t.Error("rise did not flare")
	}
}

func TestSpendWatcherIgnoresNoiseAndUnchanged(t *testing.T) {
	var w SpendWatcher
	w.Observe(today(12.400))
	if w.Observe(today(12.400)) || w.Observe(today(12.403)) {
		t.Error("unchanged or sub-cent reading flared")
	}
}

// Today's cost resets at midnight; the drop must not flare, and spend after the
// reset is measured from the new, lower baseline.
func TestSpendWatcherDropResetsBaseline(t *testing.T) {
	var w SpendWatcher
	w.Observe(today(40))
	if w.Observe(today(0)) {
		t.Error("midnight reset flared")
	}
	if !w.Observe(today(0.20)) {
		t.Error("spend after reset did not flare")
	}
}

func TestSpendWatcherSkipsUnusableSummaries(t *testing.T) {
	var w SpendWatcher
	w.Observe(today(5))
	bad := engine.Summary{Status: engine.StatusError}
	if w.Observe(bad) {
		t.Error("error summary flared")
	}
	if !w.Observe(today(5.10)) {
		t.Error("baseline was clobbered by the error summary")
	}
}
