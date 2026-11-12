//go:build !windows

// Package autostart registers the app to start when the user signs in.
package autostart

import "errors"

// Enabled is not implemented on this platform yet.
func Enabled(string) (bool, error) { return false, nil }

// Set is not implemented on this platform yet.
func Set(string, string, bool) error {
	return errors.New("autostart is not supported on this platform")
}
