// Package app is Burnt's Windows shell: the tray icon, the WebView2 popover, the
// JS bridge, the poll loop, notifications, launch-at-login and auto-update.
//
// Almost everything here is Windows-only and lives behind a build tag. The files
// without one (this one, tray_text.go, place.go, logfile.go, assets.go) hold the
// decision-making and are unit-tested on any OS, which is the only way this code
// gets tested without a Windows machine.
package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/settings"
	"github.com/mafex11/Burnt/windows/internal/update"
)

// UpdateStatus is the vocabulary ui/app.js switches on; do not rename these.
type UpdateStatus string

const (
	UpdateIdle      UpdateStatus = "idle"
	UpdateChecking  UpdateStatus = "checking"
	UpdateUpToDate  UpdateStatus = "upToDate"
	UpdateAvailable UpdateStatus = "available"
	UpdateUpdating  UpdateStatus = "updating"
	UpdateFailed    UpdateStatus = "error"
)

// UpdateInfo is the `update` branch of the bridge state. Latest is a bare version
// string ("1.4.0"): app.js renders it as "Update available — v" + latest.
type UpdateInfo struct {
	Status UpdateStatus `json:"status"`
	Latest string       `json:"latest"`
	Error  string       `json:"error"`
}

// State is the whole bridge payload. burnt_getState returns it as a JSON string and
// Go pushes the same string into window.burntOnState after every change.
type State struct {
	Summary       engine.Summary    `json:"summary"`
	Settings      settings.Settings `json:"settings"`
	Version       string            `json:"version"`
	Update        UpdateInfo        `json:"update"`
	Loading       bool              `json:"loading"`
	LaunchAtLogin bool              `json:"launchAtLogin"`
}

// JSON encodes the state for the bridge.
func (s State) JSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("app: encode state: %w", err)
	}
	return string(data), nil
}

// AfterCheck maps an update.Check outcome onto the UI's status vocabulary.
func AfterCheck(res update.Result, err error) UpdateInfo {
	if err != nil {
		return UpdateInfo{Status: UpdateFailed, Error: ShortError(err)}
	}
	latest := ""
	if res.Latest != nil {
		latest = res.Latest.Version
	}
	if res.Available && res.Latest != nil {
		return UpdateInfo{Status: UpdateAvailable, Latest: latest}
	}
	return UpdateInfo{Status: UpdateUpToDate, Latest: latest}
}

// AfterInstallFailure keeps the "an update exists" fact visible when applying it
// blew up, so the user can retry or download it by hand.
func AfterInstallFailure(prev UpdateInfo, err error) UpdateInfo {
	return UpdateInfo{Status: UpdateFailed, Latest: prev.Latest, Error: ShortError(err)}
}

// ShortError flattens an error into one tidy line. Messages reach a 320 px-wide
// popover (and summary.staleReason is rendered verbatim), so they must stay short.
func ShortError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.Join(strings.Fields(err.Error()), " ")
	const max = 140
	if len(msg) > max {
		msg = strings.TrimSpace(msg[:max-1]) + "…"
	}
	return msg
}

// UpdatesEnabled reports whether this build should talk to the GitHub releases API.
// Local and CI builds carry a placeholder version that no published tag can beat, so
// checking would only ever produce noise (or a downgrade).
func UpdatesEnabled(version string) bool {
	return version != "" && version != "dev" && !strings.HasPrefix(version, "0.0.0-dev")
}

// JSCall renders a one-string-argument JavaScript call, e.g.
//
//	JSCall("window.burntOnState", `{"a":1}`) == `window.burntOnState("{\"a\":1}")`
//
// The argument is JSON-encoded, which is exactly a JS string literal, so quotes,
// backslashes and newlines in a model or project name can't break out of it.
func JSCall(fn, arg string) string {
	quoted, err := json.Marshal(arg)
	if err != nil {
		return ""
	}
	return fn + "(" + string(quoted) + ")"
}

// DueForCheck reports whether an update check should run now. lastCheck is an RFC3339
// stamp from settings; an empty or unparseable value means "never checked", and so does
// a stamp in the future (a clock that has since been corrected).
func DueForCheck(lastCheck string, now time.Time, interval time.Duration) bool {
	if lastCheck == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, lastCheck)
	if err != nil {
		return true
	}
	if last.After(now) {
		return true
	}
	return now.Sub(last) >= interval
}

// ShowPageJS returns JavaScript that switches the dashboard to a page as soon as
// app.js has defined the hook.
//
// The retry matters: the tray's "Settings…" opens the popover and immediately asks for
// the settings page, but on the very first open the document is still loading and
// window.burntShowPage does not exist yet, so a plain call would be a no-op and the
// user would get the dashboard instead.
func ShowPageJS(page string) string {
	quoted, err := json.Marshal(page)
	if err != nil {
		return ""
	}
	return "(function r(n){var f=window.burntShowPage;" +
		"if(typeof f==='function'){f(" + string(quoted) + ");}" +
		"else if(n>0){setTimeout(function(){r(n-1);},50);}})(40)"
}
