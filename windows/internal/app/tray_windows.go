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

// flareCycles is how many times the flicker plays when spend rises (~3s): long
// enough to notice, short enough that the ticker is off almost all the time.
const flareCycles = 3

// flameAnimator owns the icon-only flame: a still frame, plus a short flicker
// burst ("flare") each time spend rises.
type flameAnimator struct {
	mu      sync.Mutex
	stop    chan struct{}
	running bool
	light   bool
	left    int // frames left in the current flare
	total   int // frames in a full flare, for extending a running one
}

// Apply shows the flame in icon-only mode. With flare set (spend just rose) and
// animate on, it flickers for one burst; a flare during a burst restarts the
// count. Otherwise it rests on the still frame, letting a running burst finish
// unless animate was turned off or the theme changed.
func (f *flameAnimator) Apply(animate, flare, light bool) {
	f.mu.Lock()
	if f.running && animate && f.light == light {
		if flare {
			f.left = f.total
		}
		f.mu.Unlock()
		return
	}
	f.stopLocked()
	f.light = light
	if !animate || !flare {
		showStillFlame(light)
		f.mu.Unlock()
		return
	}
	frames, err := trayicon.FlameFrames(light, flameFPS)
	if err != nil || len(frames) == 0 {
		log.Printf("tray: flame frames: %v", err)
		showStillFlame(light)
		f.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	f.stop, f.running = stop, true
	f.total = len(frames) * flareCycles
	f.left = f.total
	f.mu.Unlock()

	safego("flameAnimator", func() { f.runFlare(frames, stop, light) })
}

func showStillFlame(light bool) {
	if icon, err := trayicon.Flame(light); err != nil {
		log.Printf("tray: flame: %v", err)
	} else {
		systray.SetIcon(icon)
	}
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

// runFlare plays frames until the burst is used up, then rests on the still frame.
// Icons are set under f.mu so a concurrent Stop (e.g. a switch to a text mode)
// can't be overwritten by a late frame.
func (f *flameAnimator) runFlare(frames [][]byte, stop chan struct{}, light bool) {
	ticker := time.NewTicker(time.Second / flameFPS)
	defer ticker.Stop()
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		case <-ticker.C:
			f.mu.Lock()
			if f.stop != stop || !f.running {
				f.mu.Unlock()
				return
			}
			if f.left <= 0 {
				f.running = false
				showStillFlame(light)
				f.mu.Unlock()
				return
			}
			f.left--
			systray.SetIcon(frames[i%len(frames)])
			f.mu.Unlock()
		}
	}
}
