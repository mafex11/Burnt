//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// Apply stages the downloaded zip and launches the detached script that swaps
// the files in. It returns as soon as the script is running; the caller must
// then exit promptly so the swap can proceed.
func Apply(zipPath, installDir, exePath string) error {
	installDir, err := filepath.Abs(installDir)
	if err != nil {
		return fmt.Errorf("resolve install dir: %w", err)
	}
	if exePath == "" {
		exePath = filepath.Join(installDir, "burnt.exe")
	}

	updateDir := filepath.Join(installDir, updateDirName)
	if err := os.RemoveAll(updateDir); err != nil {
		return fmt.Errorf("clear staging dir: %w", err)
	}
	if err := ExtractZip(zipPath, updateDir); err != nil {
		return fmt.Errorf("stage update: %w", err)
	}

	scriptPath := filepath.Join(installDir, scriptName)
	script := buildUpdateScript(os.Getpid(), updateDir, installDir, exePath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	cmd := exec.Command("cmd", "/c", "start", "", "/min", scriptPath)
	cmd.Dir = installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.DETACHED_PROCESS,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch update script: %w", err)
	}
	// Do not Wait: the script outlives us on purpose.
	_ = cmd.Process.Release()
	return nil
}
