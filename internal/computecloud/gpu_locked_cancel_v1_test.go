//go:build linux

package computecloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func cloudTestProcessActiveV1(pid int) bool {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	i := strings.LastIndexByte(string(stat), ')')
	if i < 0 || len(stat) <= i+2 {
		return true
	}
	return stat[i+2] != 'Z' && stat[i+2] != 'X'
}

func TestCloudLockedGPUCancelKillsChildAndReleasesV1(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mock-worker")
	pidFile := filepath.Join(dir, "child.pid")
	lockDir := filepath.Join(dir, "gpu-lock")

	// Mock only: no CUDA device is accessed.
	script := fmt.Sprintf(
		"#!/bin/sh\nsleep 10 &\nchild=$!\nprintf '%%s\\n' \"$child\" > %s\nwait \"$child\"\n",
		strconv.Quote(pidFile),
	)
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256([]byte(script))
	hash := hex.EncodeToString(digest[:])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type outcome struct {
		result CloudGPUResultV1
		err    error
	}
	done := make(chan outcome, 1)

	go func() {
		result, err := RunLocalGPULabWithGlobalLockV1(
			ctx, []byte(cloudGPUSampleV1), bin, hash, lockDir,
		)
		done <- outcome{result, err}
	}()

	// Wait for the mock worker to spawn its descendant.
	deadline := time.Now().Add(3 * time.Second)
	childPID := 0
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			childPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil || childPID <= 1 {
				t.Fatalf("invalid child PID: %q", data)
			}
			break
		}
		select {
		case out := <-done:
			t.Fatalf("worker returned before ready: %v", out.err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("mock worker did not become ready")
	}

	// The worker must hold the global lock.
	lock, err := TryAcquireGlobalGPULockV1(lockDir)
	if lock != nil {
		_ = lock.Release()
		t.Fatal("concurrent GPU reservation accepted")
	}
	if !errors.Is(err, ErrGlobalGPUBusyV1) {
		t.Fatalf("expected busy lock, got: %v", err)
	}

	// Cancel while the worker and child are running.
	cancel()

	select {
	case out := <-done:
		if !errors.Is(out.err, context.Canceled) {
			t.Fatalf("expected cancellation: %v", out.err)
		}
		if !reflect.DeepEqual(out.result, CloudGPUResultV1{}) {
			t.Fatal("cancelled GPU result was accepted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled worker did not return promptly")
	}

	// Child must stop and the inherited lock must become reusable.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !cloudTestProcessActiveV1(childPID) {
			lock, err := TryAcquireGlobalGPULockV1(lockDir)
			if err == nil {
				if err := lock.Release(); err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, ErrGlobalGPUBusyV1) {
				t.Fatal(err)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("descendant or inherited GPU lock remained active")
}
