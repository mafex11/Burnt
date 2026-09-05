package engine

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: run the child with no console, so polling
// ccusage every minute never flashes a black window over the user's desktop.
const createNoWindow = 0x08000000

// hideWindow keeps the ccusage subprocess invisible.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
