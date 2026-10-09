package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPrismAPIRuntimeLockExclusive(t *testing.T) {
	dir := t.TempDir()

	first, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal(err)
	}

	if second, err := acquirePrismAPIRuntimeLock(dir); err == nil {
		_ = second.Release()
		t.Fatal("second API acquired an active lock")
	}

	if err := first.Release(); err != nil {
		t.Fatal(err)
	}

	if err := first.Release(); err != nil {
		t.Fatal("idempotent release failed:", err)
	}

	restarted, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal("restart after release failed:", err)
	}

	if err := restarted.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestPrismAPIRuntimeLockRejectsInvalidDir(t *testing.T) {
	if lock, err := acquirePrismAPIRuntimeLock(""); err == nil {
		_ = lock.Release()
		t.Fatal("empty data directory accepted")
	}

	missing := filepath.Join(t.TempDir(), "missing")

	if lock, err := acquirePrismAPIRuntimeLock(missing); err == nil {
		_ = lock.Release()
		t.Fatal("missing data directory accepted")
	}
}

// Executed only inside a dedicated child process.
func TestPrismAPIRuntimeLockProcessHelper(t *testing.T) {
	if os.Getenv("PRISM_D5_LOCK_CHILD") != "1" {
		return
	}

	dir := os.Getenv("PRISM_D5_LOCK_DIR")
	ready := os.Getenv("PRISM_D5_LOCK_READY")

	lock, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		ready, []byte("ready"), 0600,
	); err != nil {
		t.Fatal(err)
	}

	// Parent terminates this process deliberately.
	// No release runs after an abrupt kill.
	time.Sleep(30 * time.Second)

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestPrismAPIRuntimeLockAcrossProcesses(t *testing.T) {
	dir := t.TempDir()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	ready := filepath.Join(dir, "child-ready")

	cmd := exec.CommandContext(
		ctx,
		executable,
		"-test.run=^TestPrismAPIRuntimeLockProcessHelper$",
	)

	cmd.Env = append(
		os.Environ(),
		"PRISM_D5_LOCK_CHILD=1",
		"PRISM_D5_LOCK_DIR="+dir,
		"PRISM_D5_LOCK_READY="+ready,
	)

	output := &bytes.Buffer{}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	waited := false

	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	deadline := time.Now().Add(10 * time.Second)

	for {
		if _, err := os.Stat(ready); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"child did not acquire lock: %s",
				output.String(),
			)
		}

		time.Sleep(10 * time.Millisecond)
	}

	// The first independent process owns the data directory.
	if second, err := acquirePrismAPIRuntimeLock(dir); err == nil {
		_ = second.Release()
		t.Fatal("concurrent API process acquired lock")
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}

	waitErr := cmd.Wait()
	waited = true

	if waitErr == nil {
		t.Fatal("child unexpectedly exited successfully")
	}

	lockPath := filepath.Join(
		dir,
		prismAPIRuntimeLockName,
	)

	info, err := os.Stat(lockPath)
	if err != nil || !info.IsDir() {
		t.Fatal("crashed process did not leave stale lock")
	}

	// Crash recovery must fail closed.
	if next, err := acquirePrismAPIRuntimeLock(dir); err == nil {
		_ = next.Release()
		t.Fatal("stale lock was ignored")
	}

	// Only safe in this isolated test directory after
	// confirming the child process has exited.
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}

	restarted, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal("verified recovery failed:", err)
	}

	if err := restarted.Release(); err != nil {
		t.Fatal(err)
	}
}
