// Package ui holds the popover dashboard's static assets, embedded into burnt.exe.
//
// mock.js is deliberately NOT embedded: it only exists so index.html can be opened
// in an ordinary browser during development, and shipping it would let a fixture
// bridge shadow the real one. The loopback asset server answers /mock.js with an
// empty script instead (see internal/app).
package ui

import "embed"

// FS is the dashboard, served to WebView2 over loopback HTTP.
//
//go:embed index.html app.js styles.css
var FS embed.FS
