package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuantumAuditEpochJournalMonotonic(t *testing.T) {
	dir := t.TempDir()

	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)

	steps := []struct {
		name        string
		epoch       uint64
		fingerprint string
		wantError   bool
	}{
		{"Initialize", 7, first, false},
		{"RestartSameEpoch", 7, first, false},
		{"AdvanceEpoch", 8, second, false},
		{"RejectRollback", 7, first, true},
		{"RejectSameEpochPolicyChange", 8, first, true},
		{"RestartNewEpoch", 8, second, false},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			err := recordQuantumAuditPolicyEpoch(
				dir, step.epoch, step.fingerprint,
			)

			if (err != nil) != step.wantError {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}

	path := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Restarts must not append duplicate records.
	if strings.Count(string(data), "\n") != 2 {
		t.Fatal("unexpected epoch journal history")
	}

	latest, exists, _, err :=
		readQuantumAuditEpochJournal(path)

	if err != nil || !exists {
		t.Fatalf("cannot reload epoch journal: %v", err)
	}

	if latest.Epoch != 8 ||
		latest.PolicyFingerprint != second {
		t.Fatalf("unexpected persisted epoch: %+v", latest)
	}
}

func TestQuantumAuditEpochJournalCorruption(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)

	valid := `{"epoch":7,"policy_fingerprint":"` +
		fingerprint + `"}` + "\n"

	other := `{"epoch":6,"policy_fingerprint":"` +
		fingerprint + `"}` + "\n"

	cases := []struct {
		name     string
		contents string
	}{
		{"Empty", ""},
		{"Truncated", strings.TrimSuffix(valid, "\n")},
		{"MalformedJSON", "not-json\n"},
		{"DuplicateEpoch", valid + valid},
		{"DecreasingEpoch", valid + other},
		{"UnknownField", `{"epoch":7,"extra":1}` + "\n"},
		{"InvalidFingerprint",
			`{"epoch":7,"policy_fingerprint":"bad"}` + "\n"},
		{"ZeroEpoch",
			`{"epoch":0,"policy_fingerprint":"` +
				fingerprint + `"}` + "\n"},
		{"BlankLine", valid + "\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			path := filepath.Join(
				dir, quantumAuditEpochJournalName,
			)

			if err := os.WriteFile(
				path, []byte(tc.contents), 0600,
			); err != nil {
				t.Fatal(err)
			}

			err := recordQuantumAuditPolicyEpoch(
				dir, 8, fingerprint,
			)

			if err == nil {
				t.Fatal("corrupt journal was accepted")
			}
		})
	}
}

func TestQuantumAuditEpochJournalInvalidInput(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)

	cases := []struct {
		name        string
		epoch       uint64
		fingerprint string
	}{
		{"ZeroEpoch", 0, fingerprint},
		{"EmptyFingerprint", 7, ""},
		{"ShortFingerprint", 7, "abcd"},
		{"UppercaseFingerprint", 7,
			strings.Repeat("A", 64)},
		{"NonHexFingerprint", 7,
			strings.Repeat("z", 64)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			if err := recordQuantumAuditPolicyEpoch(
				dir, tc.epoch, tc.fingerprint,
			); err == nil {
				t.Fatal("invalid epoch or fingerprint accepted")
			}

			path := filepath.Join(
				dir, quantumAuditEpochJournalName,
			)

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("invalid input created an epoch journal")
			}
		})
	}
}

func TestQuantumAuditEpochJournalRejectsSymlink(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "target")
	if err := os.WriteFile(
		target, []byte("do not modify"), 0600,
	); err != nil {
		t.Fatal(err)
	}

	journal := filepath.Join(
		dir, quantumAuditEpochJournalName,
	)

	if err := os.Symlink(target, journal); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}

	err := recordQuantumAuditPolicyEpoch(
		dir, 7, strings.Repeat("a", 64),
	)

	if err == nil {
		t.Fatal("symlink journal was accepted")
	}
}
