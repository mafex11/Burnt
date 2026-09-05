package app

import (
	"log"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/mafex11/Burnt/windows/internal/trayicon"
)

// flameFPS is how fast the icon-only flame flickers. Six frames a second reads as
// alive without looking like a strobe, and each frame is an ICO systray caches to a
// temp file, so the whole animation costs six files and no per-frame rendering.
const flameFPS = 6

// trayIcon renders the ICO for a tray text produced by TrayText.
func trayIcon(text string, light bool) ([]byte, error) {
	if text == trayicon.PlaceholderText {
		return trayicon.Placeholder(), nil
	}
	return trayicon.RenderText(text, light)
}

// flameAnimator owns the icon-only flame: either a still frame or a loop, swapped
// whenever the theme or the animate setting changes.
type flameAnimator struct {
	mu      sync.Mutex
	stop    chan struct{}
	running bool
	light   bool
}

// Apply makes the tray show the flame, animated or not. It is idempotent: calling it
// with unchanged arguments while the loop is running does nothing.
func (f *flameAnimator) Apply(animate, light bool) {
	f.mu.Lock()
	if f.running && animate && f.light == light {
		f.mu.Unlock()
		return
	}
	f.stopLocked()
	f.light = light
	if !animate {
		f.mu.Unlock()
		if icon, err := trayicon.Flame(light); err != nil {
			log.Printf("tray: flame: %v", err)
		} else {
			systray.SetIcon(icon)
		}
		return
	}
	frames, err := trayicon.FlameFrames(light, flameFPS)
	if err != nil || len(frames) == 0 {
		f.mu.Unlock()
		log.Printf("tray: flame frames: %v", err)
		f.Apply(false, light)
		return
	}
	stop := make(chan struct{})
	f.stop, f.running = stop, true
	f.mu.Unlock()

	safego("flameAnimator", func() { runFlame(frames, stop) })
}

// Stop leaves the flame behind, e.g. when the user switches to a text mode.
func (f *flameAnimator) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked()
}

func (f *flameAnimator) stopLocked() {
	if f.running {
		close(f.stop)
		f.running = false
	}
}

func runFlame(frames [][]byte, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second / flameFPS)
	defer ticker.Stop()
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		case <-ticker.C:
			systray.SetIcon(frames[i%len(frames)])
		}
	}
}
