//go:build linux

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// A fresh process group allows cancellation to kill shells and ordinary
// child processes spawned by an external CUDA worker, not only its leader.
func configureGPUWorkerProcessV062(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
