//go:build !windows && !linux

package llama

import (
	"os"
	"syscall"
)

func sysProcAttr() *syscall.SysProcAttr { return nil }

// attachToJob is a no-op on this platform.
func attachToJob(*os.Process) error { return nil }
