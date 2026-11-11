//go:build !windows

package app

import "unsafe"

func cursorPos() (int, int, bool) { return 0, 0, false }

func forceForeground(unsafe.Pointer) {}
