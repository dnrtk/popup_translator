//go:build windows

// Package autostart registers the app to start when the user signs in.
package autostart

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Enabled reports whether a Run entry named name exists for the current user.
func Enabled(name string) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("open Run key: %w", err)
	}
	defer k.Close()
	_, _, err = k.GetStringValue(name)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, registry.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("read Run value: %w", err)
	}
}

// Set adds or removes the Run entry. exe is quoted in the stored command.
func Set(name, exe string, enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer k.Close()
	if enabled {
		if err := k.SetStringValue(name, `"`+exe+`"`); err != nil {
			return fmt.Errorf("write Run value: %w", err)
		}
		return nil
	}
	if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete Run value: %w", err)
	}
	return nil
}
