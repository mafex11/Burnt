# Burnt 🔥

A native macOS menu bar app that shows how much you've *burnt* on Claude Code and Codex — real-dollar cost and token usage, today and this week, glanceable from the menu bar.

## Install

```bash
brew tap mafex11/tap
brew install --cask mafex11/tap/burnt
```

**First launch:** Burnt is ad-hoc signed, so macOS may say it "cannot be opened." Right-click `Burnt.app` → **Open** → **Open** (one time only). No system Node.js needed — a self-contained Node runtime and `ccusage` are bundled inside the app.

Then `◔ $X.XX` appears in your menu bar and updates itself every 60 seconds.

## What it shows

- **Today / this week** cost in USD and token volume
- **By tool** — Claude vs Codex
- **By model** — where the expensive tokens go
- **Cache savings** — an estimate of what caching saved you

> **Billing:** Burnt is built for **API (pay-per-token)** accounts, where the `$` is real money spent. It still works on Pro/Max/Team subscriptions — the token counts are exact — but there the `$` is the **API-equivalent value** of your usage (what it *would* cost on the API), not a bill. A dedicated subscription mode is planned.

## How it works

Burnt bundles a self-contained Node runtime plus a pinned copy of [`ccusage`](https://github.com/ryoppippi/ccusage), and runs `ccusage daily --json` (which reads `~/.claude` and `~/.codex`), then aggregates the result into a glanceable summary. All cost/pricing is owned by ccusage (LiteLLM pricing table); Burnt is the native menu bar surface. Bundling Node + ccusage means no system Node install and instant startup.

Pricing is always fetched live so the numbers match `ccusage` exactly. The menu bar refreshes every 60 seconds and on every popover open.

## Build from source

```bash
git clone https://github.com/mafex11/Burnt && cd Burnt
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer swift test   # run the engine test suite
./packaging/make-app.sh   # downloads portable Node + ccusage, builds the signed .app
open Burnt.app
```

> Requires `npm` and network at build time to vendor Node + ccusage (not needed at runtime).
