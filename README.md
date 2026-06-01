<div align="center">

# Burnt 🔥

**See how much you've burnt on Claude Code and Codex — right in your menu bar.**

Real-dollar cost and token usage, today / this week / this month, at a glance.

[![Release](https://img.shields.io/github/v/release/mafex11/Burnt?color=F2A03D&label=release)](https://github.com/mafex11/Burnt/releases/latest)
[![Platform](https://img.shields.io/badge/macOS-14%2B-black?logo=apple)](https://github.com/mafex11/Burnt)
[![License](https://img.shields.io/github/license/mafex11/Burnt?color=blue)](LICENSE)

```bash
brew install --cask mafex11/tap/burnt
```

</div>

---

## Screenshots

<!-- Drop your images here. Suggested: a menu bar shot + the open popover.
     Put files in docs/images/ and reference them like below. -->

<div align="center">

<!-- ![Burnt menu bar](docs/images/menubar.png) -->
<!-- ![Burnt popover](docs/images/popover.png) -->

*(Screenshots coming soon — `🔥 $4.21` in the menu bar, click for the full dashboard.)*

</div>

```
 🔥 $4.21               ← always in your menu bar
 ┌──────────────────────────┐
 │ $4.21              ⚙ ↻   │  today  ▲ 12% vs last week
 │ ████████░░  42% of $10   │  daily budget
 │ Week    Month    All-time │
 │ $28.90  $112.40  $7,468  │
 │ ▁▃▅▇▅▃▂▁▃▅▇▅▃▂  (14-day) │  hover any bar for the day
 │ ● Claude  ▓▓▓▓▓▓░  $3.10  │
 │ ● Codex   ▓▓░░░░░  $1.11  │
 │ ≈ $12.40 saved via cache  │
 └──────────────────────────┘
```

## Why Burnt

- **Glanceable** — your daily burn lives in the menu bar; no terminal, no dashboard to open.
- **Accurate** — numbers match [`ccusage`](https://github.com/ryoppippi/ccusage) to the cent (it's bundled inside).
- **Self-contained** — no Node install, no dependencies, works offline. One `brew` command.
- **Yours to shape** — Minimal / Standard / Detailed dashboard styles, a daily budget bar, and four menu-bar display modes.

## Install

### Homebrew (recommended)

```bash
brew install --cask mafex11/tap/burnt
```

That's it — no Homebrew tap step needed (the full path taps automatically). To update later:

```bash
brew upgrade --cask mafex11/tap/burnt
```

### Manual download

1. Download **`Burnt.zip`** from the [latest release](https://github.com/mafex11/Burnt/releases/latest).
2. Unzip it and drag **`Burnt.app`** into `/Applications`.

### First launch (one-time)

Burnt is ad-hoc signed (not notarized), so on first launch macOS may say *"Burnt cannot be opened because Apple cannot check it for malicious software."* This is expected. To open it:

- **Right-click** `Burnt.app` → **Open** → **Open**, or
- **System Settings → Privacy & Security → Open Anyway**

You only need to do this once. After that, a **🔥 $X.XX** appears in your menu bar and refreshes automatically.

No system Node.js or other dependencies are required — a self-contained Node runtime and `ccusage` are bundled inside the app.

## What it shows

Click the menu bar icon for a breakdown:

- **Today / week / month / all-time** — cost in USD and token volume
- **Trend & pace** — how this week compares to last, and where today is heading
- **By tool** — Claude vs Codex, color-coded
- **By model** — where the expensive tokens go (opus, sonnet, gpt-5, …)
- **Cache savings** — an estimate of how much prompt caching saved you
- **14-day sparkline** — hover any bar for that day's date + cost

## Settings (⚙)

- **Dashboard style** — *Minimal* (just the numbers + chart), *Standard* (+ tool split), or *Detailed* (everything)
- **Menu bar shows** — Today $, Today tokens, Week $, or just the icon
- **Daily budget** — a progress bar that turns amber near your cap and red when over
- **Launch at login**

> **Billing note:** Burnt is built for **API (pay-per-token)** accounts, where the `$` is real money spent. It still works on Pro/Max/Team subscriptions — the token counts are exact — but there the `$` is the **API-equivalent value** of your usage (what it *would* cost on the API), not a bill. A dedicated subscription mode is planned.

## Requirements

- macOS 14 (Sonoma) or later
- Apple Silicon (arm64)
- [Claude Code](https://github.com/anthropics/claude-code) and/or [Codex](https://github.com/openai/codex) usage logs in `~/.claude` / `~/.codex`

## How it works

Burnt bundles a self-contained Node runtime plus a pinned copy of [`ccusage`](https://github.com/ryoppippi/ccusage) and runs `ccusage daily --json` (which reads your local `~/.claude` and `~/.codex` logs), then aggregates the result into a glanceable summary. All cost/pricing is owned by ccusage (LiteLLM pricing table); Burnt is the native menu bar surface on top of it.

Pricing is always fetched live so the numbers match `ccusage` exactly. The menu bar refreshes every 60 seconds and on every popover open. Nothing leaves your machine except ccusage's pricing-table fetch.

## Build from source

```bash
git clone https://github.com/mafex11/Burnt && cd Burnt

# Run the engine test suite (needs Xcode for XCTest)
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test

# Build the signed, self-contained .app (downloads portable Node + ccusage)
./packaging/make-app.sh
open Burnt.app
```

Cutting a release (builds, zips, stamps the cask's sha256):

```bash
./packaging/make-release.sh
gh release create vX.Y.Z Burnt.zip -R mafex11/Burnt --title "Burnt X.Y.Z" --notes "…"
```

> Building requires `npm` + network (to vendor Node + ccusage) and Xcode (for the test suite). Neither is needed at runtime.

## Project layout

```
Sources/UsageEngine/   Pure, tested Swift library: parse → classify → price → aggregate
Sources/Burnt/         SwiftUI MenuBarExtra app (thin UI over the engine)
packaging/             Icon generator, app-bundle + release scripts, Homebrew cask
Tests/                 Engine unit tests + integration smoke test
```

## License

MIT — see [LICENSE](LICENSE).
