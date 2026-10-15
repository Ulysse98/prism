//go:build !linux

package main

import (
	"fmt"
	"os/exec"
)

// This experimental CUDA subprocess backend supports Linux and WSL2 only.
// Native Windows Prism builds remain possible; CPU processing is unaffected.
func configureGPUWorkerProcessV062(cmd *exec.Cmd) error {
	return fmt.Errorf("GPU v0.62 process isolation requires Linux/WSL2")
}
