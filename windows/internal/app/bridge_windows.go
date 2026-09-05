package app

import (
	"log"

	"github.com/jchv/go-webview2"
)

// bind registers the JS bridge. Names and argument types are the contract in
// ui/README.md; app.js degrades to a no-op if one is missing, which would look like a
// silently broken dashboard, so a failed Bind is logged loudly.
//
// Every binding runs on the popover's message-pump thread, so none of them may block:
// anything slow (a ccusage run, a GitHub request, an installer) is handed to a
// goroutine that pushes fresh state when it finishes. The synchronous part only
// records the optimistic status app.js is already showing.
func (p *popover) bind(w webview2.WebView) {
	s := p.shell
	bind := func(name string, fn any) {
		if err := w.Bind(name, fn); err != nil {
			log.Printf("bridge: bind %s: %v", name, err)
		}
	}

	bind("burnt_getState", s.stateJSON)
	bind("burnt_refresh", func() { s.requestRefresh(s.wantProjects()) })
	bind("burnt_setSettings", s.applySettingsJSON)
	bind("burnt_setLaunchAtLogin", s.applyLaunchAtLogin)
	bind("burnt_getLaunchAtLogin", launchAtLoginEnabled)
	bind("burnt_checkUpdates", func() { s.startUpdateCheck() })
	bind("burnt_installUpdate", func() { s.startInstall() })
	bind("burnt_openUrl", func(url string) {
		safego("openUrl", func() {
			if err := openURL(url); err != nil {
				log.Printf("bridge: openUrl: %v", err)
			}
		})
	})
	bind("burnt_copyText", func(text string) {
		safego("copyText", func() {
			if err := setClipboardText(text); err != nil {
				log.Printf("bridge: copyText: %v", err)
			}
		})
	})
	bind("burnt_quit", s.Quit)
	bind("burnt_resize", p.Resize)
}
