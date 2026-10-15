package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// runGPUWorkerProcessV062 executes one worker with size-limited output.
// On Linux, cancellation kills the entire worker process group.
func runGPUWorkerProcessV062(ctx context.Context, binary string, request []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binary)
	if err := configureGPUWorkerProcessV062(cmd); err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewReader(request)
	// Do not wait indefinitely for inherited stdout/stderr descriptors.
	cmd.WaitDelay = 2 * time.Second

	stdout := &boundedGPUOutputV062{limit: 128 * 1024}
	stderr := &boundedGPUOutputV062{limit: 16 * 1024}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()

	if ctx.Err() != nil {
		return nil, fmt.Errorf("GPU execution timed out or cancelled: %w", ctx.Err())
	}
	if stdout.exceeded {
		return nil, fmt.Errorf("GPU stdout exceeds limit")
	}
	if stderr.exceeded {
		return nil, fmt.Errorf("GPU stderr exceeds limit")
	}
	if err != nil {
		message := string(stderr.data)
		if len(message) > 2000 {
			message = message[:2000]
		}
		return nil, fmt.Errorf("GPU process failed: %w; stderr: %s", err, message)
	}
	return stdout.data, nil
}
