//go:build !windows

package llama

import (
	"os"
	"syscall"
)

func sysProcAttr() *syscall.SysProcAttr { return nil }

// attachToJob is a no-op outside Windows (Linux will use PR_SET_PDEATHSIG).
func attachToJob(*os.Process) error { return nil }
