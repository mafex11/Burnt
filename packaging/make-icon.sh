#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Generate AppIcon.icns from a single 1024px master rendered by CoreGraphics.
# No external design tool needed.

WORK="$(mktemp -d)"
MASTER="$WORK/icon-1024.png"

# 1. Render the 1024px master with a tiny Swift/CoreGraphics program.
cat > "$WORK/render.swift" <<'SWIFT'
import AppKit

let size = 1024
let img = NSImage(size: NSSize(width: size, height: size))
img.lockFocus()
let ctx = NSGraphicsContext.current!.cgContext

// Rounded-rect background with a warm vertical gradient (orange -> deep red).
let rect = CGRect(x: 0, y: 0, width: size, height: size)
let radius: CGFloat = CGFloat(size) * 0.2237   // macOS "squircle"-ish corner
let path = CGPath(roundedRect: rect, cornerWidth: radius, cornerHeight: radius, transform: nil)
ctx.addPath(path)
ctx.clip()
let colors = [
    CGColor(red: 1.00, green: 0.58, blue: 0.20, alpha: 1), // top: amber
    CGColor(red: 0.86, green: 0.16, blue: 0.10, alpha: 1)  // bottom: ember red
] as CFArray
let grad = CGGradient(colorsSpace: CGColorSpaceCreateDeviceRGB(), colors: colors, locations: [0, 1])!
ctx.drawLinearGradient(grad, start: CGPoint(x: 0, y: size), end: CGPoint(x: 0, y: 0), options: [])

// Draw a white flame using the SF Symbol "flame.fill", centered.
// Use the symbol's palette configuration to tint it white directly (the
// sourceAtop fill trick is unreliable for multipath symbols).
let pointSize = CGFloat(size) * 0.52
var cfg = NSImage.SymbolConfiguration(pointSize: pointSize, weight: .bold)
cfg = cfg.applying(.init(paletteColors: [.white]))
if let flame = NSImage(systemSymbolName: "flame.fill", accessibilityDescription: nil)?
    .withSymbolConfiguration(cfg) {
    flame.isTemplate = false
    let fs = flame.size
    let origin = NSPoint(x: (CGFloat(size) - fs.width) / 2, y: (CGFloat(size) - fs.height) / 2)
    flame.draw(at: origin, from: NSRect(origin: .zero, size: fs), operation: .sourceOver, fraction: 1)
}

img.unlockFocus()

guard let tiff = img.tiffRepresentation,
      let rep = NSBitmapImageRep(data: tiff),
      let png = rep.representation(using: .png, properties: [:]) else {
    FileHandle.standardError.write("failed to render icon\n".data(using: .utf8)!)
    exit(1)
}
try! png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
SWIFT

DEVELOPER_DIR="${DEVELOPER_DIR:-/Applications/Xcode.app/Contents/Developer}" \
  swift "$WORK/render.swift" "$MASTER"

# 2. Build the .iconset (all required sizes) and convert to .icns.
ICONSET="$WORK/AppIcon.iconset"
mkdir -p "$ICONSET"
gen() { sips -z "$2" "$2" "$MASTER" --out "$ICONSET/$1" >/dev/null; }
gen "icon_16x16.png" 16
gen "icon_16x16@2x.png" 32
gen "icon_32x32.png" 32
gen "icon_32x32@2x.png" 64
gen "icon_128x128.png" 128
gen "icon_128x128@2x.png" 256
gen "icon_256x256.png" 256
gen "icon_256x256@2x.png" 512
gen "icon_512x512.png" 512
gen "icon_512x512@2x.png" 1024

mkdir -p packaging
iconutil -c icns "$ICONSET" -o packaging/AppIcon.icns
rm -rf "$WORK"
echo "Generated packaging/AppIcon.icns"
