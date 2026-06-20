# Burnt Website — Design

Date: 2026-06-20
Status: Approved (pending spec review)

## Goal

A single, polished marketing landing page for Burnt — the macOS menu-bar cost
tracker for Claude Code and Codex. It must look deliberately designed and modern,
explicitly NOT like a generic AI-generated dark-slate template. Primary call to
action: install via Homebrew. Secondary: GitHub, watch the demo.

## Theme (locked via visual brainstorming)

- **Layout direction:** Split hero — value copy on the left, a live-looking menu-bar
  popover mockup on the right. Below the fold, a vertical scroll of feature sections.
- **Skin:** Light editorial (warm-neutral paper, not dark mode). This is the primary
  anti-generic decision — almost every AI dev-tool site is dark.
- **Palette (cool crimson on porcelain — no ember/orange/brown):**
  - `--bg` page `#f6f4f4` (porcelain)
  - `--panel` secondary surface `#ece8e8`
  - `--card` raised surface `#ffffff`
  - `--ink` primary text `#181314`
  - `--ink-2` secondary text on chips `#332a2c`
  - `--muted` muted text `#83777a`
  - `--line` borders `#e4dcdc`
  - `--chip` input/code background `#ede6e6`
  - `--accent` `#c01933`, `--accent-bright` `#e2344b`
  - `--accent-grad` `linear-gradient(90deg,#e2344b,#c01933)`
  - `--pos` positive/green delta `#15803d`
  - Heatmap ramp (light→full): `#ede6e6,#f5cdd5,#e89aa6,#db6678,#cc3a52,#c01933`
  - A dark-section variant for one band (see Sections): ink `#181314` bg with paper text, accent unchanged.
- **Type:**
  - Headline + brand: **Manrope** (800 for h1/h2, 700 for brand/h3).
  - Body/UI: Manrope (400–600). Numerals in the popover: Manrope 700.
  - Code/commands: **JetBrains Mono** (400/500).
  - Self-hosted via `next/font/google` (Manrope, JetBrains Mono) — no external CDN
    request at runtime, no layout shift.
- **Logo:** the existing flame mark, recolored to `--accent`. Inline SVG (the path
  used throughout the mockups), set beside the wordmark "Burnt" in Manrope 700.

## Stack & architecture

- **Next.js (App Router) + Tailwind CSS**, TypeScript. Static — the whole page is
  server-rendered/exported; no client data fetching. Near-zero runtime JS (only the
  copy-command button and the demo video are interactive).
- **Location:** new top-level directory `site/` inside the Burnt repo (its own
  `package.json`; does not touch the Swift app or its build).
- **Design tokens:** the palette above lives as CSS variables in `app/globals.css`
  and is mapped into `tailwind.config.ts` (`colors: { ink, paper, panel, accent, … }`)
  so components use semantic class names, not raw hex.
- **Deployment:** static export (`output: 'export'`) so it can host on GitHub Pages
  or Vercel. (Hosting choice is out of scope for this spec; the build must produce a
  static `out/`.)

### Component structure (`site/`)

Each section is one focused component under `components/`:

- `Nav.tsx` — sticky top bar: flame + "Burnt" wordmark left; links (Features, Install,
  GitHub) right; subtle bottom border that appears on scroll. A small "macOS 14+" chip.
- `Hero.tsx` — the split. Left: eyebrow ("Menu-bar cost tracker"), h1 with the italic-
  accent word "burnt", one-line lede, install command chip + copy button, GitHub link.
  Right: `PopoverMock`.
- `PopoverMock.tsx` — a static, pixel-faithful recreation of the app popover: today's
  `$` with a green ↑ delta, week/month sub-line, three breakdown bars, a 12-week
  heatmap grid. Pure CSS/SVG, no real data.
- `LogoStrip.tsx` — a thin "works with" line: Claude Code · Codex · powered by ccusage.
- `Features.tsx` — a bento/section grid of the README's value props, each with a tiny
  inline visual: Today/Week/Month/All-time, Trend & pace, 14-day sparkline, 12-week
  heatmap, By tool, By model, By project, Cache savings, Burnt Wrapped.
- `Demo.tsx` — the launch video (`burnt-launch.mp4`, `muted loop playsinline`, poster
  = `hero.png`) framed in a faux macOS window chrome. One dark band for contrast.
- `Install.tsx` — large `brew install mafex11/tap/burnt` block with copy button;
  manual-download fallback line; the one-time first-launch note (right-click → Open).
- `Footer.tsx` — wordmark, links (GitHub, Releases, License MIT), "made by" line, the
  "How much have you burnt today?" sign-off.
- `CopyButton.tsx` — the one client component: copies a command to clipboard, swaps
  label to "Copied ✓" for 1.5s.

### Assets (reuse from the repo)

- Demo video: `docs/video/burnt-launch.mp4` → copy to `site/public/burnt-launch.mp4`.
- Poster / hero still: `docs/images/hero.png` → `site/public/hero.png`.
- App icon: `docs/video/appicon.png` → `site/public/appicon.png` (favicon + OG image).
- Flame logo: inline SVG component (not a file).

## Content (source of truth: README.md)

- **H1:** "Know what you've *burnt*." (accent on "burnt")
- **Lede:** "Real-dollar cost and token usage for Claude Code and Codex — today, this
  week, this month — right in your menu bar."
- **Value props (3 pillars, from README "Why Burnt"):** Glanceable · Accurate
  (matches ccusage to the cent) · Self-contained (no Node, one brew command).
- **Feature grid:** the eight README "What it shows" bullets, verbatim in spirit.
- **Install:** `brew install mafex11/tap/burnt`; update line `brew upgrade --cask
  mafex11/tap/burnt`; manual download + first-launch right-click→Open note.
- **Sign-off:** "How much have you burnt today?"

No fabricated metrics, testimonials, or logos. Numbers shown in the popover/feature
mockups are illustrative and labeled as a preview, not real telemetry.

## Responsive & accessibility

- Breakpoints: the hero split stacks to single column < 900px (copy first, popover
  below). Bento collapses to 1–2 columns on mobile.
- Color contrast: ink `#181314` on paper `#f6f4f4` and white on `--accent` both exceed
  WCAG AA. The crimson is used for accents/headlined words, not body text.
- Motion: the demo video autoplays muted+looped; respect `prefers-reduced-motion`
  (pause / show poster). All interactive elements keyboard-focusable with visible focus.
- Semantic HTML: one `<h1>`, sections with headings, `alt` text on imagery.

## Testing / verification

- `npm run build` (static export) succeeds with zero errors; `out/` is produced.
- Manual visual check across desktop + mobile widths (the dev server, screenshot review).
- Lighthouse pass target: Performance ≥ 95, Accessibility ≥ 95 (static page, self-
  hosted fonts, one small video → easily achievable).
- Verify the copy button copies the exact install command.
- Verify no external font/CDN requests at runtime (fonts self-hosted via next/font).

## Out of scope (YAGNI)

- Docs/FAQ pages, blog, the future web dashboard app shell (separate roadmap item).
- Analytics, cookie banners, newsletter capture.
- Dark-mode toggle (the page is intentionally light; one dark band is for contrast only).
- A backend of any kind — fully static.
