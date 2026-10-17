package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGPUProcessGroupTimeoutV062(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 process groups required")
	}
	dir := t.TempDir()
	childPIDPath := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "fake-gpu")
	// The shell and sleep inherit the dedicated GPU process group.
	body := fmt.Sprintf("#!/bin/sh\nsleep 30 &\nprintf '%%s\\n' \"$!\" > %s\nwait\n", strconv.Quote(childPIDPath))
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runGPUWorkerProcessV062(ctx, script, []byte("{}"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("GPU process cancellation took too long")
	}
	raw, err := os.ReadFile(childPIDPath)
	if err != nil {
		t.Fatalf("fake GPU did not spawn its child: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 1 {
		t.Fatalf("invalid child PID: %q (%v)", raw, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// A zombie is no longer running; PID 1 may reap it later.
		stat, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if errors.Is(readErr, os.ErrNotExist) || errors.Is(readErr, syscall.ESRCH) {
			t.Log("PASS: GPU descendant terminated after timeout")
			return
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		idx := bytes.LastIndex(stat, []byte(") "))
		if idx >= 0 && len(stat) > idx+2 && (stat[idx+2] == 'Z' || stat[idx+2] == 'X') {
			t.Log("PASS: GPU descendant exited after timeout (zombie pending reaping)")
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("GPU descendant PID %d still running after timeout", pid)
}
