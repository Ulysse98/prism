//go:build linux && amd64

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGPUSharedLockV066ExclusionAndRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	first, err := tryAcquireSharedGPULockV066(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := tryAcquireSharedGPULockV066(dir)
	if second != nil || !errors.Is(err, errSharedGPUBusyV066) {
		t.Fatalf("concurrent flock accepted: %v", err)
	}
	if err := first.file.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := tryAcquireSharedGPULockV066(dir)
	if err != nil {
		t.Fatalf("lock not recovered after close: %v", err)
	}
	defer third.file.Close()
}

func TestGPUSharedLockV066UnsafePaths(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"relative", dir + "/../other"} {
		if lock, err := tryAcquireSharedGPULockV066(path); err == nil {
			_ = lock.file.Close()
			t.Fatalf("unsafe path accepted: %s", path)
		}
	}
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	if lock, err := tryAcquireSharedGPULockV066(link); err == nil {
		_ = lock.file.Close()
		t.Fatal("symlink directory accepted")
	}
	insecure := filepath.Join(dir, "insecure")
	if err := os.Mkdir(insecure, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(insecure, 0755); err != nil {
		t.Fatal(err)
	}
	if lock, err := tryAcquireSharedGPULockV066(insecure); err == nil {
		_ = lock.file.Close()
		t.Fatal("non-private directory accepted")
	}
	unsafeFile := filepath.Join(realDir, sharedGPULockFileV066)
	if err := os.Symlink(filepath.Join(dir, "target"), unsafeFile); err != nil {
		t.Fatal(err)
	}
	if lock, err := tryAcquireSharedGPULockV066(realDir); err == nil {
		_ = lock.file.Close()
		t.Fatal("symlink lock file accepted")
	}
}

// Helper emulates a separate cooperating process using the Cloud-compatible protocol.
func TestGPUSharedLockOwnerHelperV066(t *testing.T) {
	dir := os.Getenv("PRISM_V066_LOCK_OWNER_HELPER")
	if dir == "" {
		return
	}
	lock, err := tryAcquireSharedGPULockV066(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.file.Close()
	if err := os.WriteFile(os.Getenv("PRISM_V066_LOCK_OWNER_READY"), []byte("READY"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		time.Sleep(25 * time.Millisecond)
	}
}

func TestGPUSharedLockV066CrashRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestGPUSharedLockOwnerHelperV066$")
	cmd.Env = append(os.Environ(), "PRISM_V066_LOCK_OWNER_HELPER="+dir, "PRISM_V066_LOCK_OWNER_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lock owner did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if lock, err := tryAcquireSharedGPULockV066(dir); lock != nil || !errors.Is(err, errSharedGPUBusyV066) {
		if lock != nil {
			_ = lock.file.Close()
		}
		t.Fatalf("failed cross-process exclusion: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lock, err := tryAcquireSharedGPULockV066(dir)
		if err == nil {
			_ = lock.file.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("GPU flock not recovered after owner kill")
}

// The sealed child checks inherited FD 4 and waits until its supervisor
// has closed its descriptor. No GPU required.
func TestGPUSharedLockSealedChildHelperV066(t *testing.T) {
	ready := os.Getenv("PRISM_V066_SEALED_CHILD_READY")
	if ready == "" {
		return
	}
	if _, err := os.Stat("/proc/self/fd/4"); err != nil {
		t.Fatalf("child did not inherit GPU lock FD 4: %v", err)
	}
	if err := os.WriteFile(ready, []byte("READY"), 0600); err != nil {
		t.Fatal(err)
	}
	release := os.Getenv("PRISM_V066_SEALED_CHILD_RELEASE")
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("parent did not release sealed child")
}

func TestGPUSharedLockV066SealedChildSurvivesParentClose(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	sha := sha256.Sum256(data)
	digest := hex.EncodeToString(sha[:])
	sealed, err := newSealedGPUWorkerV066(exe, digest, maxGPUExecutableBytesV066)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	dir := filepath.Join(t.TempDir(), "shared")
	lock, err := tryAcquireSharedGPULockV066(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.file.Close()
	control := t.TempDir()
	ready, release := filepath.Join(control, "ready"), filepath.Join(control, "release")
	t.Setenv("PRISM_V066_SEALED_CHILD_READY", ready)
	t.Setenv("PRISM_V066_SEALED_CHILD_RELEASE", release)
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		_, e := runSealedGPUWorkerWithSharedLockStartedV066(context.Background(), sealed, lock.file, nil, started, "-test.run=^TestGPUSharedLockSealedChildHelperV066$")
		done <- e
	}()
	select {
	case <-started:
	case e := <-done:
		t.Fatalf("sealed child failed to start: %v", e)
	case <-time.After(6 * time.Second):
		t.Fatal("sealed child start timed out")
	}
	deadline := time.Now().Add(6 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case e := <-done:
			t.Fatalf("sealed child failed before readiness: %v", e)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("sealed child did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Parent closes, *without LOCK_UN*. Child must keep ownership.
	if err := lock.file.Close(); err != nil {
		t.Fatal(err)
	}
	contender, err := tryAcquireSharedGPULockV066(dir)
	if contender != nil || !errors.Is(err, errSharedGPUBusyV066) {
		if contender != nil {
			_ = contender.file.Close()
		}
		t.Fatalf("sealed child lost inherited GPU ownership: %v", err)
	}
	if err := os.WriteFile(release, []byte("GO"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatalf("sealed child failed: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("sealed child did not exit")
	}
	recovered, err := tryAcquireSharedGPULockV066(dir)
	if err != nil {
		t.Fatalf("GPU ownership not recovered after worker exit: %v", err)
	}
	defer recovered.file.Close()
}

func TestGPUSharedLockV066InvalidWorkerFailsClosed(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "shared")
	_, err = runApprovedGPUWorkerWithSharedLockV066(context.Background(), exe, strings.Repeat("0", 64), maxGPUExecutableBytesV066, nil, dir)
	if err == nil {
		t.Fatal("unapproved worker executed")
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("unapproved worker acquired GPU reservation: %v", statErr)
	}
}
