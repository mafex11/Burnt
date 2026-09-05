package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	d := Default()
	if d.MenuBarMode != ModeTodayCost {
		t.Errorf("menuBarMode = %q, want todayCost", d.MenuBarMode)
	}
	if d.DashboardStyle != StyleStandard {
		t.Errorf("dashboardStyle = %q, want standard", d.DashboardStyle)
	}
	if d.DailyBudget != 0 {
		t.Errorf("dailyBudget = %v, want 0", d.DailyBudget)
	}
	if !d.AnimateFlame || !d.AutoUpdate {
		t.Errorf("animateFlame/autoUpdate should default on, got %v/%v", d.AnimateFlame, d.AutoUpdate)
	}
	if d.NotifyBudget || d.NotifyDailySummary || d.NotifyMilestones {
		t.Error("notifications should default off")
	}
	if d.LastUpdateCheck != "" {
		t.Errorf("lastUpdateCheck = %q, want empty", d.LastUpdateCheck)
	}
}

func TestJSONKeys(t *testing.T) {
	data, err := json.Marshal(Default())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{
		"menuBarMode", "dailyBudget", "dashboardStyle", "notifyBudget",
		"notifyDailySummary", "notifyMilestones", "animateFlame", "autoUpdate",
		"lastUpdateCheck",
	} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing json key %q", k)
		}
	}
	if len(m) != 9 {
		t.Errorf("got %d keys, want 9: %v", len(m), m)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope", "settings.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if got != Default() {
		t.Errorf("got %+v, want defaults", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "settings.json")
	want := Settings{
		MenuBarMode:        ModeWeekCost,
		DailyBudget:        12.5,
		DashboardStyle:     StyleDetailed,
		NotifyBudget:       true,
		NotifyDailySummary: true,
		NotifyMilestones:   true,
		AnimateFlame:       false,
		AutoUpdate:         false,
		LastUpdateCheck:    "2026-09-06T10:00:00Z",
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.DailyBudget = 40
	if err := Save(path, s); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.DailyBudget != 40 {
		t.Errorf("dailyBudget = %v, want 40", got.DailyBudget)
	}
	// The temp file must not linger next to the real one.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only settings.json, got %v", entries)
	}
}

func TestLoadPartialFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"dailyBudget": 25, "notifyBudget": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Default()
	want.DailyBudget = 25
	want.NotifyBudget = true
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadUnknownKeysIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"whoDis": 1, "animateFlame": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AnimateFlame {
		t.Error("animateFlame should be false")
	}
	if got.MenuBarMode != ModeTodayCost {
		t.Errorf("menuBarMode = %q, want default", got.MenuBarMode)
	}
}

func TestLoadCorruptFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for corrupt JSON")
	}
	if got != Default() {
		t.Errorf("on error should return defaults, got %+v", got)
	}
}

func TestLoadCoercesBadEnums(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	body := `{"menuBarMode":"lolwut","dashboardStyle":"maximal","dailyBudget":-5}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.MenuBarMode != ModeTodayCost {
		t.Errorf("menuBarMode = %q, want todayCost", got.MenuBarMode)
	}
	if got.DashboardStyle != StyleStandard {
		t.Errorf("dashboardStyle = %q, want standard", got.DashboardStyle)
	}
	if got.DailyBudget != 0 {
		t.Errorf("dailyBudget = %v, want 0", got.DailyBudget)
	}
}

func TestValidateKeepsGoodValues(t *testing.T) {
	for _, m := range []MenuBarMode{ModeTodayCost, ModeTodayTokens, ModeWeekCost, ModeIconOnly} {
		s := Default()
		s.MenuBarMode = m
		if got := s.Validate().MenuBarMode; got != m {
			t.Errorf("Validate() changed %q to %q", m, got)
		}
	}
	for _, st := range []DashboardStyle{StyleMinimal, StyleStandard, StyleDetailed} {
		s := Default()
		s.DashboardStyle = st
		if got := s.Validate().DashboardStyle; got != st {
			t.Errorf("Validate() changed %q to %q", st, got)
		}
	}
}

func TestDashboardStyleOrdering(t *testing.T) {
	if !StyleDetailed.AtLeast(StyleStandard) || !StyleStandard.AtLeast(StyleMinimal) {
		t.Error("style ranks should increase minimal < standard < detailed")
	}
	if StyleMinimal.AtLeast(StyleStandard) {
		t.Error("minimal should not satisfy standard")
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "settings.json" || filepath.Base(filepath.Dir(p)) != "Burnt" {
		t.Errorf("DefaultPath() = %q, want .../Burnt/settings.json", p)
	}
}
