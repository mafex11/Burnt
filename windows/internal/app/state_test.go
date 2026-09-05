package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/settings"
	"github.com/mafex11/Burnt/windows/internal/update"
)

// The bridge contract in ui/README.md is a JSON shape, so assert the shape, not the
// Go struct: a renamed tag would silently blank half the dashboard.
func TestStateJSONShape(t *testing.T) {
	st := State{
		Summary:       engine.Summary{Status: engine.StatusOK},
		Settings:      settings.Default(),
		Version:       "1.3.0",
		Update:        UpdateInfo{Status: UpdateAvailable, Latest: "1.4.0"},
		Loading:       true,
		LaunchAtLogin: true,
	}
	raw, err := st.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"summary", "settings", "version", "update", "loading", "launchAtLogin"} {
		if _, ok := got[key]; !ok {
			t.Errorf("state JSON missing %q: %s", key, raw)
		}
	}
	upd, ok := got["update"].(map[string]any)
	if !ok {
		t.Fatalf("update is not an object: %s", raw)
	}
	if upd["status"] != "available" || upd["latest"] != "1.4.0" {
		t.Errorf("update = %v, want status available / latest 1.4.0", upd)
	}
	if got["loading"] != true || got["launchAtLogin"] != true {
		t.Errorf("loading/launchAtLogin lost: %s", raw)
	}
}

func TestAfterCheck(t *testing.T) {
	rel := &update.Release{Tag: "v1.4.0", Version: "1.4.0"}

	if got := AfterCheck(update.Result{Available: true, Latest: rel}, nil); got.Status != UpdateAvailable || got.Latest != "1.4.0" {
		t.Errorf("available: got %+v", got)
	}
	if got := AfterCheck(update.Result{Latest: rel}, nil); got.Status != UpdateUpToDate {
		t.Errorf("up to date: got %+v", got)
	}
	got := AfterCheck(update.Result{}, errors.New("dial tcp: no route to host"))
	if got.Status != UpdateFailed || got.Error == "" {
		t.Errorf("error: got %+v", got)
	}
	// Available with no release is nonsense; treat it as "nothing to install".
	if got := AfterCheck(update.Result{Available: true}, nil); got.Status != UpdateUpToDate {
		t.Errorf("available without release: got %+v", got)
	}
}

func TestAfterInstallFailureKeepsVersion(t *testing.T) {
	prev := UpdateInfo{Status: UpdateUpdating, Latest: "1.4.0"}
	got := AfterInstallFailure(prev, errors.New("copy failed"))
	if got.Status != UpdateFailed || got.Latest != "1.4.0" || got.Error != "copy failed" {
		t.Errorf("got %+v", got)
	}
}

func TestShortError(t *testing.T) {
	if got := ShortError(nil); got != "" {
		t.Errorf("nil error = %q", got)
	}
	if got := ShortError(errors.New("line one\n  line two")); got != "line one line two" {
		t.Errorf("newlines not flattened: %q", got)
	}
	long := ShortError(errors.New(strings.Repeat("x", 400)))
	if len([]rune(long)) > 141 || !strings.HasSuffix(long, "…") {
		t.Errorf("not truncated: %d runes", len([]rune(long)))
	}
}

func TestUpdatesEnabled(t *testing.T) {
	cases := map[string]bool{
		"1.3.0":             true,
		"1.3.0-beta.1":      true,
		"dev":               false,
		"":                  false,
		"0.0.0-dev.abc1234": false,
	}
	for version, want := range cases {
		if got := UpdatesEnabled(version); got != want {
			t.Errorf("UpdatesEnabled(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestJSCallEscapes(t *testing.T) {
	if got := JSCall("window.burntOnState", `{"a":1}`); got != `window.burntOnState("{\"a\":1}")` {
		t.Errorf("got %s", got)
	}

	// A project name carrying a quote, a newline and a closing script tag must stay
	// inside the string literal — this JSON reaches the page as source code.
	payload := "{\"project\":\"a\\\"</script>\\nb\"}"
	got := JSCall("window.burntOnState", payload)
	if !strings.HasPrefix(got, `window.burntOnState("`) || !strings.HasSuffix(got, `")`) {
		t.Fatalf("malformed call: %s", got)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("raw newline survived: %q", got)
	}
	// The literal decodes back to exactly what we passed in.
	var back string
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(got, "window.burntOnState("), ")")), &back); err != nil {
		t.Fatalf("literal is not valid JSON: %v", err)
	}
	if back != payload {
		t.Errorf("round trip changed the payload:\n got %q\nwant %q", back, payload)
	}
}

func TestDueForCheck(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	cases := []struct {
		name string
		last string
		want bool
	}{
		{"never checked", "", true},
		{"garbage", "yesterday", true},
		{"an hour ago", now.Add(-time.Hour).Format(time.RFC3339), false},
		{"exactly a day ago", now.Add(-day).Format(time.RFC3339), true},
		{"a week ago", now.Add(-7 * day).Format(time.RFC3339), true},
		{"in the future", now.Add(time.Hour).Format(time.RFC3339), true},
	}
	for _, tc := range cases {
		if got := DueForCheck(tc.last, now, day); got != tc.want {
			t.Errorf("%s: DueForCheck(%q) = %v, want %v", tc.name, tc.last, got, tc.want)
		}
	}
}

func TestShowPageJS(t *testing.T) {
	got := ShowPageJS("settings")
	if !strings.Contains(got, `f("settings")`) {
		t.Errorf("page name missing or unquoted: %s", got)
	}
	if !strings.Contains(got, "setTimeout") {
		t.Error("no retry: the first open races the document load")
	}
	// The page name is a JS string literal, so a hostile value stays inert.
	if strings.Contains(ShowPageJS(`");alert(1);//`), `alert(1)`) &&
		!strings.Contains(ShowPageJS(`");alert(1);//`), `\"`) {
		t.Error("page name is not escaped")
	}
}
