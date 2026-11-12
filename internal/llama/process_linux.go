//go:build linux

package llama

import (
	"os"
	"syscall"
)

// sysProcAttr asks the kernel to SIGKILL llama-server when the parent dies
// (the Linux counterpart of the Windows Job Object).
//
// Caveat: PR_SET_PDEATHSIG fires when the *thread* that forked the child
// exits. Go may retire OS threads, so a future Linux implementation should
// start llama-server from a goroutine pinned with runtime.LockOSThread that
// lives as long as the process (see docs/FINDINGS.md, Linux).
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}

func attachToJob(*os.Process) error { return nil }
