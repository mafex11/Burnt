package app

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/mafex11/Burnt/windows/internal/engine"
	"github.com/mafex11/Burnt/windows/internal/settings"
	"github.com/mafex11/Burnt/windows/internal/update"
	"github.com/mafex11/Burnt/windows/ui"
)

const (
	pollInterval    = 60 * time.Second
	refreshTimeout  = 90 * time.Second // two ccusage runs at 30 s each, plus slack
	webview2HelpURL = "https://github.com/mafex11/Burnt#windows"
)

// Run is the whole Windows app. It blocks until the user quits, and returns the
// process exit code.
//
// The main goroutine is given to systray.Run, which needs it: systray pumps its own
// Win32 message loop and its window must live on the thread that created it (its
// package init already calls runtime.LockOSThread). Everything else — the poll loop,
// the menu, updates, and the WebView2 popover on its own locked thread — hangs off it.
func Run(version string) int {
	if !claimSingleInstance() {
		// A Burnt is already in the tray. Starting a second one would give the user
		// two icons and two pollers, so this one bows out silently.
		return 0
	}

	s, closeLog := newShell(version)
	defer closeLog()

	log.Printf("burnt %s starting (pid %d)", version, os.Getpid())
	systray.SetOnTapped(func() { safego("tray.tapped", s.TogglePopover) })
	systray.Run(s.onReady, s.onExit)
	return 0
}

// shell holds everything the tray, the popover and the background loops share.
type shell struct {
	version      string
	dataDir      string
	settingsPath string

	engine *engine.Engine
	notif  *notifier
	assets *assetServer
	pop    *popover
	flame  *flameAnimator
	client *http.Client

	menu struct {
		open, today, week, refresh, settings, updates, quit *systray.MenuItem
	}

	// trayMu serialises tray redraws: a settings change arrives on the popover's
	// thread and can otherwise race the poll loop's redraw.
	trayMu sync.Mutex

	mu     sync.Mutex
	st     State
	latest *update.Release // newest release seen, for burnt_installUpdate

	refreshReq chan bool
	done       chan struct{}
	quitOnce   sync.Once
}

func newShell(version string) (*shell, func()) {
	settingsPath, err := settings.DefaultPath()
	if err != nil {
		// Nowhere to persist preferences; run on defaults rather than not at all.
		settingsPath = ""
	}
	dataDir := filepath.Dir(settingsPath)
	if settingsPath == "" {
		dataDir = filepath.Join(os.TempDir(), "Burnt")
	}

	closeLog := startLogging(filepath.Join(dataDir, "burnt.log"))

	cfg := settings.Default()
	if settingsPath != "" {
		loaded, err := settings.Load(settingsPath)
		if err != nil {
			log.Printf("settings: %v (using defaults)", err)
		}
		cfg = loaded
	}

	s := &shell{
		version:      version,
		dataDir:      dataDir,
		settingsPath: settingsPath,
		notif:        newNotifier(dataDir),
		flame:        &flameAnimator{},
		client:       &http.Client{Timeout: 20 * time.Second},
		refreshReq:   make(chan bool, 1),
		done:         make(chan struct{}),
	}
	s.st = State{
		// Start as noData, not a zero Summary: an empty Status would make the tray
		// render "$0.00" for the second before the first ccusage run lands.
		Summary:       engine.Summary{Status: engine.StatusNoData},
		Settings:      cfg,
		Version:       version,
		Update:        UpdateInfo{Status: UpdateIdle},
		LaunchAtLogin: launchAtLoginEnabled(),
	}

	// A nil runner is legal: the Engine then reports StatusError, which the tray shows
	// as "—" and the popover as an error line, and Burnt keeps running so the user can
	// read the message instead of watching the app vanish.
	inv, err := engine.Locator{}.Resolve()
	if err != nil {
		log.Printf("engine: %v", err)
		s.engine = engine.New(nil)
	} else {
		log.Printf("engine: using %s", inv.Executable)
		s.engine = engine.New(engine.NewRunner(inv))
	}

	return s, closeLog
}

// startLogging redirects the standard logger to %APPDATA%\Burnt\burnt.log. Burnt is a
// -H windowsgui binary: it has no console, so anything not written here is lost.
func startLogging(path string) func() {
	f, err := openLogFile(path, MaxLogBytes)
	if err != nil {
		log.SetOutput(io.Discard)
		return func() {}
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	return func() { f.Close() }
}

/* ------------------------------------------------------------------ lifecycle */

func (s *shell) onReady() {
	defer func() {
		if r := recover(); r != nil {
			logPanic("onReady", r)
		}
	}()

	srv, err := startAssetServer(ui.FS)
	if err != nil {
		log.Printf("assets: %v", err)
	} else {
		s.assets = srv
	}
	url := ""
	if s.assets != nil {
		url = s.assets.URL
	}
	s.pop = newPopover(s, url, filepath.Join(s.dataDir, "WebView2"))

	s.buildMenu()
	s.applyTray()

	safego("menuLoop", s.menuLoop)
	safego("refreshLoop", s.refreshLoop)
	safego("updateLoop", s.updateLoop)
}

func (s *shell) onExit() {
	close(s.done)
	if s.pop != nil {
		s.pop.Close()
	}
	if s.assets != nil {
		s.assets.Close()
	}
	s.flame.Stop()
	log.Print("burnt exiting")
}

// Quit tears the app down. systray.Quit unwinds systray.Run on the main goroutine,
// which calls onExit.
func (s *shell) Quit() {
	s.quitOnce.Do(func() {
		if s.pop != nil {
			s.pop.hide()
		}
		safego("quit", systray.Quit)
	})
}

/* ----------------------------------------------------------------------- menu */

func (s *shell) buildMenu() {
	m := &s.menu
	m.open = systray.AddMenuItem("Open Burnt", "Show today's spend")
	systray.AddSeparator()
	m.today = systray.AddMenuItem("Today: —", "")
	m.week = systray.AddMenuItem("Week: —", "")
	m.today.Disable()
	m.week.Disable()
	systray.AddSeparator()
	m.refresh = systray.AddMenuItem("Refresh", "Fetch usage now")
	m.settings = systray.AddMenuItem("Settings…", "Preferences")
	m.updates = systray.AddMenuItem("Check for Updates…", "")
	systray.AddSeparator()
	m.quit = systray.AddMenuItem("Quit Burnt", "")
}

func (s *shell) menuLoop() {
	m := &s.menu
	for {
		select {
		case <-s.done:
			return
		case <-m.open.ClickedCh:
			s.OpenPopover(nil)
		case <-m.refresh.ClickedCh:
			s.requestRefresh(s.wantProjects())
		case <-m.settings.ClickedCh:
			s.OpenPopover(func() { s.pop.eval(ShowPageJS("settings")) })
		case <-m.updates.ClickedCh:
			s.startUpdateCheck()
		case <-m.quit.ClickedCh:
			s.Quit()
		}
	}
}

/* -------------------------------------------------------------------- popover */

// TogglePopover is the tray icon's left click.
func (s *shell) TogglePopover() {
	if s.pop == nil {
		return
	}
	if s.pop.IsVisible() {
		s.pop.hide()
		return
	}
	s.OpenPopover(nil)
}

// OpenPopover shows the dashboard and refreshes it, running then once it is up.
// WebView2 creation takes a moment, so this never runs on a message-pump thread.
func (s *shell) OpenPopover(then func()) {
	safego("openPopover", func() {
		if s.pop == nil {
			return
		}
		if err := s.pop.show(); err != nil {
			log.Printf("popover: %v", err)
			s.webView2Missing(err)
			return
		}
		if then != nil {
			then()
		}
		s.pushState()
		s.requestRefresh(s.wantProjects())
	})
}

// webView2Missing explains the one failure that leaves Burnt without a dashboard.
func (s *shell) webView2Missing(cause error) {
	log.Printf("popover: no dashboard: %v", cause)
	s.notif.Post("Burnt needs WebView2",
		"Install the Microsoft Edge WebView2 runtime to see the dashboard. Opening the instructions.")
	if err := openURL(webview2HelpURL); err != nil {
		log.Printf("popover: could not open %s: %v", webview2HelpURL, err)
	}
}

/* -------------------------------------------------------------------- refresh */

// wantProjects reports whether the next refresh should include per-project data.
func (s *shell) wantProjects() bool {
	visible := s.pop != nil && s.pop.IsVisible()
	return IncludeProjects(s.snapshot().Settings, visible)
}

// requestRefresh asks for a refresh without waiting for it. The channel holds one
// pending request, so a burst of clicks collapses into a single ccusage run.
func (s *shell) requestRefresh(includeProjects bool) {
	select {
	case s.refreshReq <- includeProjects:
	default:
	}
}

func (s *shell) refreshLoop() {
	s.refresh(false) // first paint: the cheap variant, as the popover isn't up yet
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.refresh(s.wantProjects())
		case includeProjects := <-s.refreshReq:
			s.refresh(includeProjects)
		}
	}
}

func (s *shell) refresh(includeProjects bool) {
	s.setLoading(true)

	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	summary := s.engine.LoadSummary(ctx, includeProjects)

	s.mu.Lock()
	s.st.Summary = summary
	s.st.Loading = false
	cfg := s.st.Settings
	s.mu.Unlock()

	s.applyTray()
	s.pushState()
	s.notif.Evaluate(summary, cfg, time.Now)
}

// setLoading flags the spinner. State is only pushed when the dashboard is on screen —
// a background poll shouldn't repaint a window nobody is looking at.
func (s *shell) setLoading(loading bool) {
	s.mu.Lock()
	s.st.Loading = loading
	s.mu.Unlock()
	if s.pop != nil && s.pop.IsVisible() {
		s.pushState()
	}
}

/* ------------------------------------------------------------------------ tray */

// applyTray redraws the icon, tooltip and the two disabled menu rows. The theme is
// re-read every time: the user can flip light/dark while Burnt is running.
func (s *shell) applyTray() {
	s.trayMu.Lock()
	defer s.trayMu.Unlock()

	st := s.snapshot()
	light := systemUsesLightTheme()

	if text := TrayText(st.Summary, st.Settings.MenuBarMode); text == "" {
		s.flame.Apply(AnimateFlame(st.Settings), light)
	} else {
		s.flame.Stop()
		if icon, err := trayIcon(text, light); err != nil {
			log.Printf("tray: render %q: %v", text, err)
		} else {
			systray.SetIcon(icon)
		}
	}

	systray.SetTooltip(Tooltip(st.Summary))
	today, week := MenuTotals(st.Summary)
	if s.menu.today != nil {
		s.menu.today.SetTitle(today)
		s.menu.week.SetTitle(week)
	}
}

/* -------------------------------------------------------------------- settings */

func (s *shell) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st
}

// stateJSON backs burnt_getState.
func (s *shell) stateJSON() string {
	raw, err := s.snapshot().JSON()
	if err != nil {
		log.Printf("state: %v", err)
		return ""
	}
	return raw
}

// pushState hands the current state to the dashboard.
func (s *shell) pushState() {
	if s.pop == nil {
		return
	}
	raw, err := s.snapshot().JSON()
	if err != nil {
		log.Printf("state: %v", err)
		return
	}
	s.pop.eval(JSCall("window.burntOnState", raw))
}

// applySettingsJSON backs burnt_setSettings: the UI sends the whole settings object
// on every change and there is no Save button.
//
// It deliberately does NOT push state back in the normal case. The daily-budget field
// commits on every keystroke without re-rendering (a re-render would drop the caret),
// so echoing the settings would fight the user's typing. State is only pushed when
// Validate had to correct something, which means the UI is showing a value that isn't
// what got saved.
func (s *shell) applySettingsJSON(raw string) {
	var incoming settings.Settings
	if err := json.Unmarshal([]byte(raw), &incoming); err != nil {
		log.Printf("settings: bad payload: %v", err)
		return
	}

	s.mu.Lock()
	prev := s.st.Settings
	if incoming.LastUpdateCheck == "" {
		incoming.LastUpdateCheck = prev.LastUpdateCheck // not a UI field
	}
	cleaned := incoming.Validate()
	s.st.Settings = cleaned
	s.mu.Unlock()

	s.save(cleaned)

	if cleaned.MenuBarMode != prev.MenuBarMode || cleaned.AnimateFlame != prev.AnimateFlame {
		s.applyTray()
	}
	// Switching to Detailed needs project data the last poll didn't fetch.
	if cleaned.DashboardStyle != prev.DashboardStyle {
		s.requestRefresh(s.wantProjects())
	}
	if cleaned != incoming {
		s.pushState()
	}
}

func (s *shell) save(cfg settings.Settings) {
	if s.settingsPath == "" {
		return
	}
	if err := settings.Save(s.settingsPath, cfg); err != nil {
		log.Printf("settings: save: %v", err)
	}
}

// applyLaunchAtLogin backs burnt_setLaunchAtLogin.
func (s *shell) applyLaunchAtLogin(on bool) {
	if err := setLaunchAtLogin(on); err != nil {
		log.Printf("launch at login: %v", err)
	}
	actual := launchAtLoginEnabled()
	s.mu.Lock()
	s.st.LaunchAtLogin = actual
	s.mu.Unlock()
	if actual != on {
		s.pushState() // the registry write didn't take; correct the toggle
	}
}

/* ------------------------------------------------------------------- panics */

// safego runs fn on a new goroutine, logging any panic instead of taking the whole
// tray down with it.
func safego(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logPanic(name, r)
			}
		}()
		fn()
	}()
}

func logPanic(name string, r any) {
	log.Printf("panic in %s: %v\n%s", name, r, debug.Stack())
}
