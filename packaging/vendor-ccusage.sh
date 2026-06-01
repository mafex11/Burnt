#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Vendor a self-contained Node runtime + the pinned ccusage package, so the app
# runs ccusage with NO system Node dependency.
#
# Why bundle Node instead of a bun-compiled binary: ccusage 20.x depends on a
# native module that `bun build --compile` cannot bundle (the compiled binary
# produces empty output). The official Node binary from nodejs.org is portable
# (links only system libs), so we ship it alongside ccusage's JS.

PINNED="20.0.6"        # MUST match CcusageRunner.pinnedVersion
NODE_VER="v22.22.0"    # bundled Node runtime
ARCH="darwin-arm64"
VENDOR="packaging/vendor"

rm -rf "$VENDOR"
mkdir -p "$VENDOR"

# 1. Fetch the official, portable Node binary.
echo "Downloading Node ${NODE_VER} (${ARCH})…"
TARBALL="node-${NODE_VER}-${ARCH}"
curl -fsSL "https://nodejs.org/dist/${NODE_VER}/${TARBALL}.tar.gz" -o "$VENDOR/node.tgz"
tar xzf "$VENDOR/node.tgz" -C "$VENDOR"
cp "$VENDOR/${TARBALL}/bin/node" "$VENDOR/node"
chmod +x "$VENDOR/node"
rm -rf "$VENDOR/node.tgz" "$VENDOR/${TARBALL}"

# 2. Install the pinned ccusage package (production deps only).
echo "Installing ccusage@${PINNED}…"
TMP="$(mktemp -d)"
( cd "$TMP" && npm init -y >/dev/null 2>&1 && npm install --omit=dev "ccusage@${PINNED}" >/dev/null 2>&1 )
cp -R "$TMP/node_modules" "$VENDOR/node_modules"
rm -rf "$TMP"

# 3. Smoke-check: bundled node runs ccusage and emits JSON.
echo "Verifying bundled runtime…"
"$VENDOR/node" "$VENDOR/node_modules/ccusage/dist/cli.js" --version
echo "Vendored Node ${NODE_VER} + ccusage ${PINNED} → $VENDOR/"
