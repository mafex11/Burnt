# Burnt Auto-Update — Design

Date: 2026-06-19
Status: Approved (pending spec review)

## Problem

Burnt ships frequently (five releases in its first cycle). Users who installed via
`brew install mafex11/tap/burnt` only get new versions when they manually run
`brew upgrade`, so most run stale builds. We want Burnt to keep itself current with
minimal friction and zero new trust surface.

## Decisions

1. **Channel — Homebrew is the single source of truth.** No Sparkle, no in-app
   binary self-replacement, no EdDSA signing key, no hosted appcast. Burnt never
   rewrites its own bytes; it asks `brew` to do the upgrade. This eliminates the
   classic Sparkle-vs-brew conflict (Sparkle swapping the app out from under brew's
   checksum) and avoids all code-signing/permissions complexity.
2. **Trigger — in-app.** Burnt already runs in the menu bar all day. It performs the
   version check itself and shells out to brew. No separately installed launchd
   artifact to manage or clean up.
3. **UX — silent auto-apply with a Settings toggle (default ON), plus a manual
   "Check for Updates" button.**
4. **Cadence — once per day (24h), wall-clock based**, plus a check on launch.

## Architecture

One new pure component + one new shell-out component, mirroring the existing
`CcusageLocator` (pure) / `CcusageRunner` (process) split.

### `UpdateChecker` (UsageEngine, pure, unit-tested)

- `currentVersion: String` — from `Bundle.main` `CFBundleShortVersionString` (e.g. `1.2.1`).
- `latestVersion() throws -> String` — fetches the raw cask file and parses its
  `version "X.Y.Z"` line. The cask is the exact artifact brew installs, so it is the
  truest "what would I get if I upgraded" source (preferred over the GitHub Releases
  API, which can drift from what the cask pins).
  - URL: `https://raw.githubusercontent.com/mafex11/homebrew-tap/main/Casks/burnt.rb`
  - Parse: regex `version "([0-9]+\.[0-9]+\.[0-9]+)"`.
- `compare(current:latest:) -> UpdateStatus` — semantic version comparison returning
  `.upToDate` or `.updateAvailable(String)`. Pure; the core of the unit tests.
- The network fetch is injected (a `@Sendable (URL) throws -> Data` closure) so tests
  feed fixture cask text and never touch the network.

```swift
public enum UpdateStatus: Sendable, Equatable {
    case upToDate
    case updateAvailable(String)   // the newer version string
}
```

### `Updater` (BurntCore, shell-out)

- `isBrewManaged() -> Bool` — true iff this install lives under a Homebrew Caskroom
  (checks for the cask receipt, e.g. `…/Caskroom/burnt/.metadata/INSTALL_RECEIPT.json`).
  A direct-download user returns false and we never run a brew command that would fail.
- `brewPath() -> String?` — locate `brew` at `/opt/homebrew/bin/brew` then
  `/usr/local/bin/brew`.
- `upgrade()` — runs `brew update && brew upgrade --cask burnt` as a detached
  background `Process`. brew's existing cask `postflight` already strips quarantine,
  and quitting/replacing/relaunching is handled by the cask `uninstall quit:` + reopen,
  so the new version comes back up on its own.

### `Settings` (BurntCore)

- New `@Published var autoUpdate: Bool`, persisted in UserDefaults under key
  `autoUpdate`, **default ON** when unset (same pattern as `animateFlame`).

### `AppModel` wiring (app)

- New published state: `updateStatus` (idle / checking / upToDate / updateAvailable /
  updating) so the popover/settings can show inline feedback.
- `lastUpdateCheck: Date` persisted in UserDefaults. On launch, if ≥24h has elapsed
  (or never checked), run a check; otherwise schedule the next one. A wall-clock check
  (not a pure uptime timer) so a Mac that slept overnight checks promptly on wake.
- `checkForUpdates(userInitiated:)` — ONE method shared by the daily timer and the
  manual button:
  1. `UpdateChecker.latestVersion()` off the main thread.
  2. `compare`. Stamp `lastUpdateCheck`.
  3. If `.updateAvailable` AND `settings.autoUpdate` AND `Updater.isBrewManaged()` →
     `Updater.upgrade()`.
  4. If `.updateAvailable` but auto-update off → publish "Update available — vX.Y"
     (no apply).
  5. The manual button always surfaces a result ("You're up to date" / "Update
     available — vX.Y"), even when auto-update is off. The button does NOT force-apply
     when auto-update is off — it respects the toggle and tells the user a version is
     available so they can `brew upgrade` themselves. (Apply-on-click only happens
     because step 3 fires when the toggle is on.)

### `SettingsView` (app)

- Toggle: **"Automatically update Burnt"** (bound to `settings.autoUpdate`).
- Button: **"Check for Updates"** → `model.checkForUpdates(userInitiated: true)`, with
  inline status text driven by `model.updateStatus`.

## Data flow

```
launch / 24h timer / manual button
  → UpdateChecker.latestVersion()   (network, off-main)
  → compare(current, latest)
  → newer? AND autoUpdate? AND brew-managed?
       → Updater.upgrade()  →  brew replaces app  →  relaunch on new version
  → else publish status for the popover/settings
```

## Error handling

Auto-update is best-effort and must never block or nag:

- Network failure, non-200, parse failure → log, treat as `.upToDate` for this cycle,
  retry next interval. Manual button shows a neutral "Couldn't check right now."
- `brew` not found / not brew-managed → skip the upgrade silently; a manual check still
  reports whether a newer version exists so direct-download users know to update.
- `brew upgrade` non-zero exit → log; the app keeps running the current version.

## Testing

- **`UpdateChecker` (pure):** version-compare table (equal, patch/minor/major bumps,
  current ahead of latest), and cask parsing from fixture `burnt.rb` text via the
  injected fetcher. No network.
- **`Updater`:** `isBrewManaged()` against a temp receipt path (present/absent).
- **Manual:** the real `brew upgrade --cask burnt` shell-out is integration-verified by
  hand (same approach as the ccusage subprocess), since it mutates the installed app.

## Out of scope

- Sparkle / direct self-update (rejected — conflicts with brew, adds signing surface).
- A separate launchd updater (rejected — extra installed artifact; the app runs anyway).
- Update channels / beta opt-in (YAGNI).
