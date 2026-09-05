package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mafex11/Burnt/windows/internal/update"
)

const (
	updateInterval  = 24 * time.Hour
	checkTimeout    = 30 * time.Second
	downloadTimeout = 10 * time.Minute
)

// updateLoop checks GitHub on launch (when the last check is over a day old) and once
// a day after that. Dev and CI builds never check: their version can't be compared
// against a published tag in any useful way.
func (s *shell) updateLoop() {
	if !UpdatesEnabled(s.version) {
		log.Printf("updates: disabled for version %q", s.version)
		return
	}
	if DueForCheck(s.snapshot().Settings.LastUpdateCheck, time.Now(), updateInterval) {
		s.checkUpdates()
	}
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.checkUpdates()
		}
	}
}

// startUpdateCheck backs burnt_checkUpdates and the tray's "Check for Updates…". It
// flips the status synchronously (so burnt_getState right afterwards shows "Checking…")
// and does the network call on a goroutine.
func (s *shell) startUpdateCheck() {
	s.setUpdate(UpdateInfo{Status: UpdateChecking})
	safego("checkUpdates", s.checkUpdates)
}

// startInstall backs burnt_installUpdate.
func (s *shell) startInstall() {
	info := s.snapshot().Update
	s.setUpdate(UpdateInfo{Status: UpdateUpdating, Latest: info.Latest})
	safego("installUpdate", func() { s.install(s.pendingRelease()) })
}

func (s *shell) checkUpdates() {
	s.setUpdate(UpdateInfo{Status: UpdateChecking})
	s.pushState()

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	res, err := update.Check(ctx, s.client, update.DefaultAPIURL, s.version)
	if err != nil {
		log.Printf("updates: %v", err)
	}
	s.rememberRelease(res.Latest)
	s.setUpdate(AfterCheck(res, err))
	s.recordCheckTime()
	s.pushState()

	if res.Available && res.Latest != nil && s.snapshot().Settings.AutoUpdate {
		log.Printf("updates: auto-installing %s", res.Latest.Version)
		s.setUpdate(UpdateInfo{Status: UpdateUpdating, Latest: res.Latest.Version})
		s.pushState()
		s.install(res.Latest)
	}
}

// install downloads the release zip and hands the swap to update.Apply, which spawns a
// detached script that waits for this process to exit. Quitting is therefore the last
// step, and the script relaunches the new burnt.exe.
func (s *shell) install(rel *update.Release) {
	if rel == nil || rel.AssetURL == "" {
		s.failUpdate(errors.New("no downloadable release; check for updates first"))
		return
	}

	zipPath := filepath.Join(os.TempDir(), update.AssetName)
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	downloader := &http.Client{Timeout: downloadTimeout}
	if err := update.Download(ctx, downloader, rel.AssetURL, zipPath); err != nil {
		s.failUpdate(err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		s.failUpdate(err)
		return
	}
	if err := update.Apply(zipPath, filepath.Dir(exe), exe); err != nil {
		s.failUpdate(err)
		return
	}
	log.Printf("updates: applying %s, quitting", rel.Version)
	s.Quit()
}

func (s *shell) failUpdate(err error) {
	log.Printf("updates: %v", err)
	s.mu.Lock()
	prev := s.st.Update
	s.mu.Unlock()
	s.setUpdate(AfterInstallFailure(prev, err))
	s.pushState()
}

func (s *shell) setUpdate(info UpdateInfo) {
	s.mu.Lock()
	s.st.Update = info
	s.mu.Unlock()
}

// rememberRelease keeps the asset URL of the newest release seen, so
// burnt_installUpdate has something to download without checking again.
func (s *shell) rememberRelease(rel *update.Release) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rel != nil {
		s.latest = rel
	}
}

func (s *shell) pendingRelease() *update.Release {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latest
}

// recordCheckTime persists lastUpdateCheck so a relaunch doesn't re-check immediately.
func (s *shell) recordCheckTime() {
	s.mu.Lock()
	s.st.Settings.LastUpdateCheck = time.Now().UTC().Format(time.RFC3339)
	cfg := s.st.Settings
	s.mu.Unlock()
	s.save(cfg)
}
