package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This helper runs only in a separately launched test process.
func TestQuantumAuditEpochProcessHelper(t *testing.T) {
	if os.Getenv("PRISM_C3_CHILD") != "1" {
		return
	}

	ready := os.Getenv("PRISM_C3_READY")
	gate := os.Getenv("PRISM_C3_GATE")
	dir := os.Getenv("PRISM_C3_DIR")
	fingerprint := os.Getenv("PRISM_C3_FINGERPRINT")

	if err := os.WriteFile(
		ready, []byte("ready"), 0600,
	); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)

	for {
		if _, err := os.Stat(gate); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}

		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for process gate")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err := recordQuantumAuditPolicyEpoch(
		dir, 8, fingerprint,
	); err != nil {
		// Expected exit code for a conflicting startup.
		os.Exit(13)
	}

	os.Exit(0)
}

func TestQuantumAuditEpochProcessContention(t *testing.T) {
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

	contextWithTimeout, cancel := context.WithTimeout(
		context.Background(), 15*time.Second,
	)
	defer cancel()

	gate := filepath.Join(dir, "release-children")

	type child struct {
		cmd    *exec.Cmd
		output *bytes.Buffer
		ready  string
	}

	var children []child

	for index, fingerprint := range []string{
		first, second,
	} {
		ready := filepath.Join(
			dir, "ready-"+string(rune('0'+index)),
		)

		cmd := exec.CommandContext(
			contextWithTimeout,
			executable,
			"-test.run=^TestQuantumAuditEpochProcessHelper$",
		)

		cmd.Env = append(
			os.Environ(),
			"PRISM_C3_CHILD=1",
			"PRISM_C3_READY="+ready,
			"PRISM_C3_GATE="+gate,
			"PRISM_C3_DIR="+dir,
			"PRISM_C3_FINGERPRINT="+fingerprint,
		)

		output := &bytes.Buffer{}
		cmd.Stdout = output
		cmd.Stderr = output

		if err := cmd.Start(); err != nil {
			t.Fatal("cannot start independent process:", err)
		}

		children = append(children, child{
			cmd:    cmd,
			output: output,
			ready:  ready,
		})
	}

	deadline := time.Now().Add(10 * time.Second)

	// Wait until BOTH independent processes are ready.
	for {
		readyCount := 0

		for _, process := range children {
			if _, err := os.Stat(process.ready); err == nil {
				readyCount++
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}

		if readyCount == len(children) {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("independent processes did not become ready")
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Release both processes through the same filesystem gate.
	if err := os.WriteFile(
		gate, []byte("go"), 0600,
	); err != nil {
		t.Fatal(err)
	}

	successes := 0
	conflicts := 0

	for _, process := range children {
		err := process.cmd.Wait()

		if err == nil {
			successes++
			continue
		}

		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) &&
			exitErr.ExitCode() == 13 {
			conflicts++
			continue
		}

		t.Fatalf(
			"unexpected child failure: %v\n%s",
			err,
			process.output.String(),
		)
	}

	if successes != 1 || conflicts != 1 {
		t.Fatalf(
			"expected one success and one rejection, got %d/%d",
			successes, conflicts,
		)
	}

	journal := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)

	latest, exists, _, err :=
		readQuantumAuditEpochJournal(journal)

	if err != nil || !exists || latest.Epoch != 8 {
		t.Fatalf(
			"invalid journal after contention: %+v, %v",
			latest, err,
		)
	}

	if latest.PolicyFingerprint != first &&
		latest.PolicyFingerprint != second {
		t.Fatal("unexpected policy fingerprint")
	}

	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(string(data), "\n") != 2 {
		t.Fatal("journal contains duplicate or missing records")
	}

	lock := journal + ".lock"

	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("cross-process lock remained after completion")
	}
}
