//go:build !windows

package engine

import "os/exec"

// hideWindow is a no-op off Windows: there is no console to hide.
func hideWindow(cmd *exec.Cmd) {}
