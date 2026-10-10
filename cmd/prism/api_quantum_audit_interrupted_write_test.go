package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuantumAuditInterruptedAppendFailsClosed(t *testing.T) {
	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)

	cases := []struct {
		name   string
		suffix string
	}{
		{
			name:   "PartialJSON",
			suffix: `{"epoch":8,"policy_fingerprint":"`,
		},
		{
			name: "CompleteJSONMissingNewline",
			suffix: `{"epoch":8,"policy_fingerprint":"` +
				second + `"}`,
		},
		{
			name: "MalformedCompletedRecord",
			suffix: `{"epoch":8,"policy_fingerprint":"bad"}` +
				"\n",
		},
		{
			name: "DuplicateEpoch",
			suffix: `{"epoch":7,"policy_fingerprint":"` +
				first + `"}` + "\n",
		},
		{
			name:   "UnexpectedBlankRecord",
			suffix: "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			if err := recordQuantumAuditPolicyEpoch(
				dir, 7, first,
			); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(
				dir, quantumAuditEpochJournalName,
			)

			file, err := os.OpenFile(
				path, os.O_WRONLY|os.O_APPEND, 0600,
			)
			if err != nil {
				t.Fatal(err)
			}

			written, writeErr :=
				file.Write([]byte(tc.suffix))

			if writeErr == nil && written != len(tc.suffix) {
				t.Fatal("short simulated write")
			}

			if writeErr != nil {
				t.Fatal(writeErr)
			}

			if err := file.Sync(); err != nil {
				t.Fatal(err)
			}

			if err := file.Close(); err != nil {
				t.Fatal(err)
			}

			corrupted, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			if _, _, _, err :=
				readQuantumAuditEpochJournal(path); err == nil {
				t.Fatal("corrupt history was accepted")
			}

			if err := recordQuantumAuditPolicyEpoch(
				dir, 8, second,
			); err == nil {
				t.Fatal("startup accepted corrupt history")
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(corrupted, after) {
				t.Fatal("failed startup modified corrupt history")
			}

			lock := path + ".lock"

			if _, err := os.Stat(lock); !os.IsNotExist(err) {
				t.Fatal("failed startup left a lock behind")
			}
		})
	}
}
