#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Ensure the bundled Node + ccusage exist (vendor them if missing).
if [ ! -x packaging/vendor/node ] || [ ! -f packaging/vendor/node_modules/ccusage/dist/cli.js ]; then
  ./packaging/vendor-ccusage.sh
fi

DEVELOPER_DIR="${DEVELOPER_DIR:-/Applications/Xcode.app/Contents/Developer}" swift build -c release
APP="Burnt.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp .build/release/Burnt "$APP/Contents/MacOS/Burnt"
cp packaging/Info.plist "$APP/Contents/Info.plist"

# Bundle the self-contained Node runtime + ccusage package. CcusageLocator runs
# Resources/node against Resources/node_modules/ccusage/dist/cli.js.
cp packaging/vendor/node "$APP/Contents/Resources/node"
chmod +x "$APP/Contents/Resources/node"
cp -R packaging/vendor/node_modules "$APP/Contents/Resources/node_modules"

# Ad-hoc codesign so Gatekeeper allows local launch. --deep covers the bundled node.
codesign --force --deep --sign - "$APP"
echo "Built $APP (with bundled Node + ccusage)"
