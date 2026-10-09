package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuantumAuditEpochCrashHelper(t *testing.T) {
	if os.Getenv("PRISM_C3_CRASH_CHILD") != "1" {
		return
	}

	dir := os.Getenv("PRISM_C3_CRASH_DIR")
	if dir == "" {
		t.Fatal("missing crash test directory")
	}

	lock := filepath.Join(
		dir, quantumAuditEpochJournalName+".lock",
	)

	if err := os.Mkdir(lock, 0700); err != nil {
		t.Fatal(err)
	}

	// Simulate abrupt process termination without cleanup.
	os.Exit(23)
}

func TestQuantumAuditEpochCrashRecovery(t *testing.T) {
	dir := t.TempDir()

	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)

	if err := recordQuantumAuditPolicyEpoch(
		dir, 7, first,
	); err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	child := exec.Command(
		executable,
		"-test.run=^TestQuantumAuditEpochCrashHelper$",
	)

	child.Env = append(
		os.Environ(),
		"PRISM_C3_CRASH_CHILD=1",
		"PRISM_C3_CRASH_DIR="+dir,
	)

	output, err := child.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) ||
		exitErr.ExitCode() != 23 {
		t.Fatalf(
			"unexpected child exit: %v; output: %s",
			err, output,
		)
	}

	journal := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)
	lock := journal + ".lock"

	info, err := os.Stat(lock)
	if err != nil || !info.IsDir() {
		t.Fatal("crash did not preserve the stale lock")
	}

	// The journal must fail closed after the crash.
	if err := recordQuantumAuditPolicyEpoch(
		dir, 8, second,
	); err == nil {
		t.Fatal("stale lock failed to block startup")
	}

	previous, exists, _, err :=
		readQuantumAuditEpochJournal(journal)

	if err != nil || !exists ||
		previous.Epoch != 7 ||
		previous.PolicyFingerprint != first {
		t.Fatalf(
			"journal changed after refused startup: %+v, %v",
			previous, err,
		)
	}

	// Manual recovery is safe here because the child has
	// already exited and this directory is test-only.
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}

	if err := recordQuantumAuditPolicyEpoch(
		dir, 8, second,
	); err != nil {
		t.Fatal("recovery failed:", err)
	}

	latest, exists, _, err :=
		readQuantumAuditEpochJournal(journal)

	if err != nil || !exists ||
		latest.Epoch != 8 ||
		latest.PolicyFingerprint != second {
		t.Fatalf(
			"journal invalid after recovery: %+v, %v",
			latest, err,
		)
	}

	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("recovery left a journal lock behind")
	}
}
