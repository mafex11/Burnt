#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

PINNED="20.0.6"   # MUST match CcusageRunner.pinnedVersion
OUT="packaging/vendor/ccusage"
mkdir -p packaging/vendor

if ! command -v bun >/dev/null 2>&1; then
  echo "ERROR: bun is required to vendor ccusage. Install: brew install oven-sh/bun/bun" >&2
  exit 1
fi

# Install the pinned ccusage CLI into a temp dir, then compile its entrypoint
# into a standalone, Node-free binary.
TMP="$(mktemp -d)"
( cd "$TMP" && bun add "ccusage@${PINNED}" >/dev/null 2>&1 )

# Resolve the CLI entrypoint from package.json `bin` (falls back to dist/index.js).
ENTRY="$(node -e "const p=require('$TMP/node_modules/ccusage/package.json'); const b=p.bin; const rel=typeof b==='string'?b:Object.values(b)[0]; process.stdout.write(rel)" 2>/dev/null || echo "dist/index.js")"
ENTRY_PATH="$TMP/node_modules/ccusage/$ENTRY"
echo "ccusage entrypoint: $ENTRY_PATH"

bun build --compile --minify "$ENTRY_PATH" --outfile "$OUT"
chmod +x "$OUT"
echo "Vendored ccusage ${PINNED} → $OUT"
"$OUT" --version || true   # smoke check
