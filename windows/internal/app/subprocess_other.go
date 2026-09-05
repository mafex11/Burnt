//go:build !windows

package app

import "os/exec"

// HideConsole is a no-op off Windows: there is no console to hide.
func HideConsole(cmd *exec.Cmd) {}
