#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Build a release: assemble the signed .app, zip it, and stamp the cask's sha256.
# After running this, attach Burnt.zip to a GitHub release tagged v<version>.

# 1. Build the signed, self-contained app.
DEVELOPER_DIR="${DEVELOPER_DIR:-/Applications/Xcode.app/Contents/Developer}" ./packaging/make-app.sh

# 2. Zip it (ditto preserves the code signature + resource forks).
rm -f Burnt.zip
ditto -c -k --sequesterRsrc --keepParent Burnt.app Burnt.zip

# 3. Compute the sha256 and write it into the cask.
SHA="$(shasum -a 256 Burnt.zip | awk '{print $1}')"
/usr/bin/sed -i '' -E "s|sha256 \"[a-f0-9]*\"|sha256 \"${SHA}\"|" packaging/burnt.rb

SIZE="$(ls -lh Burnt.zip | awk '{print $5}')"
echo "Built Burnt.zip (${SIZE}), sha256 ${SHA}"
echo "Cask updated. Next:"
echo "  gh release create v\$(grep -m1 version packaging/burnt.rb | cut -d'\"' -f2) Burnt.zip -R mafex11/Burnt --title ... --notes ..."
