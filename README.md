# Burnt 🔥

A native macOS menu bar app that shows how much you've *burnt* on **Claude Code** and **Codex** — real-dollar cost and token usage, today and this week, glanceable from the menu bar.

```
 🔥 $4.21          ← always in your menu bar
 ┌─────────────────────┐
 │ Today      $4.21     │
 │ This week  $28.90    │
 │ ▁▃▅▇▅▃▂  (7-day)     │
 │ Claude  $3.10  1.2M  │
 │ Codex   $1.11  340K  │
 │ ≈ $12.40 saved (cache)│
 └─────────────────────┘
```

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

- **Today / this week** — cost in USD and token volume
- **By tool** — Claude vs Codex
- **By model** — where the expensive tokens go (opus, sonnet, gpt-5, …)
- **Cache savings** — an estimate of how much prompt caching saved you

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
