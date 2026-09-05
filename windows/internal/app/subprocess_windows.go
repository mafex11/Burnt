package app

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW.
const createNoWindow = 0x08000000

// HideConsole makes a child process run without a console window. Burnt is built with
// -H windowsgui and spawns helpers (ccusage --version, the clipboard fallback), so
// without this a black console flashes over the user's desktop.
func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
