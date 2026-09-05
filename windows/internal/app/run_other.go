//go:build !windows

package app

import "fmt"

// Run is a stub off Windows. The flags in cmd/burnt (--version, --render-icon,
// --diagnose) work everywhere so the whole binary can be vetted, built and smoke
// tested on macOS and Linux, but the tray itself is Win32-only.
func Run(version string) int {
	fmt.Println("Burnt's tray app runs on Windows only")
	return 1
}
