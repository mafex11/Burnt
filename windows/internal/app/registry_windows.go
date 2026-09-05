package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	themeKeyPath = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "Burnt"
)

// systemUsesLightTheme reports whether the taskbar is light, which decides whether
// the tray text is drawn near-black or white. Absent value or any error means dark:
// that is Windows' own default and the far more common setup.
func systemUsesLightTheme() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, themeKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("SystemUsesLightTheme")
	if err != nil {
		return false
	}
	return value == 1
}

// launchAtLoginEnabled reports whether the Run key points at this executable.
func launchAtLoginEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue(runValueName)
	return err == nil
}

// setLaunchAtLogin adds or removes the Run entry. The command is quoted because the
// default install path (%LOCALAPPDATA%\Programs\Burnt) is fine but a user-chosen one
// may well contain spaces.
func setLaunchAtLogin(on bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("app: open Run key: %w", err)
	}
	defer key.Close()

	if !on {
		if err := key.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("app: clear Run value: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("app: locate executable: %w", err)
	}
	if resolved, err := filepath.Abs(exe); err == nil {
		exe = resolved
	}
	if err := key.SetStringValue(runValueName, `"`+exe+`"`); err != nil {
		return fmt.Errorf("app: set Run value: %w", err)
	}
	return nil
}
