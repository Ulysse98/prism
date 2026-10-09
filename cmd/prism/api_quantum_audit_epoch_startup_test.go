package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"prism/internal/usefulwork"
)

func quantumAuditStartupTestPolicy() *usefulwork.QuantumQuorumPolicy {
	return &usefulwork.QuantumQuorumPolicy{
		Model: "bell-ideal-v1",
		AuthorizedVerifiers: []string{
			"prism_" + strings.Repeat("a", 40),
			"prism_" + strings.Repeat("b", 40),
			"prism_" + strings.Repeat("c", 40),
		},
		RequiredApprovals: 2,
	}
}

func TestConfigureQuantumAuditEpoch(t *testing.T) {
	dir := t.TempDir()

	policy := quantumAuditStartupTestPolicy()

	if err := configureQuantumAuditEpoch(
		dir, nil, 0,
	); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled audit unexpectedly created a journal")
	}

	if err := configureQuantumAuditEpoch(
		dir, policy, 7,
	); err != nil {
		t.Fatal(err)
	}

	if err := configureQuantumAuditEpoch(
		dir, policy, 7,
	); err != nil {
		t.Fatal("identical restart was rejected:", err)
	}

	changed := *policy
	changed.RequiredApprovals = 3

	if err := configureQuantumAuditEpoch(
		dir, &changed, 7,
	); err == nil {
		t.Fatal("changed policy accepted at the same epoch")
	}

	if err := configureQuantumAuditEpoch(
		dir, &changed, 8,
	); err != nil {
		t.Fatal("new policy epoch was rejected:", err)
	}

	if err := configureQuantumAuditEpoch(
		dir, policy, 7,
	); err == nil {
		t.Fatal("startup accepted policy rollback")
	}

	if err := configureQuantumAuditEpoch(
		dir, policy, 0,
	); err == nil {
		t.Fatal("zero epoch accepted")
	}

	if err := configureQuantumAuditEpoch(
		dir, nil, 8,
	); err == nil {
		t.Fatal("epoch accepted without a policy")
	}

	latest, exists, _, err :=
		readQuantumAuditEpochJournal(path)

	if err != nil || !exists || latest.Epoch != 8 {
		t.Fatalf("invalid persisted epoch: %+v, %v", latest, err)
	}
}

func TestQuantumAuditEpochConcurrentStartup(t *testing.T) {
	dir := t.TempDir()

	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)

	if err := recordQuantumAuditPolicyEpoch(
		dir, 7, first,
	); err != nil {
		t.Fatal(err)
	}

	var accepted atomic.Int32
	var wait sync.WaitGroup

	start := make(chan struct{})

	for _, fingerprint := range []string{first, second} {
		wait.Add(1)

		go func(value string) {
			defer wait.Done()
			<-start

			if err := recordQuantumAuditPolicyEpoch(
				dir, 8, value,
			); err == nil {
				accepted.Add(1)
			}
		}(fingerprint)
	}

	close(start)
	wait.Wait()

	if accepted.Load() != 1 {
		t.Fatalf(
			"expected exactly one startup accepted, got %d",
			accepted.Load(),
		)
	}

	path := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)

	latest, exists, _, err :=
		readQuantumAuditEpochJournal(path)

	if err != nil || !exists || latest.Epoch != 8 {
		t.Fatalf(
			"concurrent startup corrupted history: %+v, %v",
			latest, err,
		)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(string(data), "\n") != 2 {
		t.Fatal("concurrent startup appended unexpected records")
	}
}

func TestQuantumAuditEpochRejectsExistingLock(t *testing.T) {
	dir := t.TempDir()

	lockPath := filepath.Join(
		dir,
		quantumAuditEpochJournalName+".lock",
	)

	if err := os.Mkdir(lockPath, 0700); err != nil {
		t.Fatal(err)
	}

	err := recordQuantumAuditPolicyEpoch(
		dir, 7, strings.Repeat("a", 64),
	)

	if err == nil {
		t.Fatal("startup accepted an occupied journal lock")
	}

	journal := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)

	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatal("occupied lock permitted journal creation")
	}
}
