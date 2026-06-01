#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Ensure the bundled ccusage exists (vendor it if missing).
if [ ! -x packaging/vendor/ccusage ]; then
  ./packaging/vendor-ccusage.sh
fi

DEVELOPER_DIR="${DEVELOPER_DIR:-/Applications/Xcode.app/Contents/Developer}" swift build -c release
APP="Burnt.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp .build/release/Burnt "$APP/Contents/MacOS/Burnt"
cp packaging/Info.plist "$APP/Contents/Info.plist"
cp packaging/vendor/ccusage "$APP/Contents/Resources/ccusage"   # the bundled binary CcusageLocator finds
chmod +x "$APP/Contents/Resources/ccusage"

# Ad-hoc codesign so Gatekeeper allows local launch. --deep covers the bundled binary.
codesign --force --deep --sign - "$APP"
echo "Built $APP (with bundled ccusage)"
