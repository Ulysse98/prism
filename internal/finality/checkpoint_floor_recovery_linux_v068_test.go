//go:build linux

package finality

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV068RecoveryInspectionReadOnlyLifecycle(t *testing.T) {
	parent, validators, signed, policy, floor, old := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
		t.Fatal(err)
	}

	state, err := InspectCheckpointFloorRecoveryV068(dir, old)
	if err != nil || state != CheckpointFloorReadyV068 {
		t.Fatalf("ready expected: %q %v", state, err)
	}

	next, err := PrepareCheckpointFloorAdvanceV068(dir, parent, validators, signed, policy, old)
	if err != nil {
		t.Fatal(err)
	}

	state, err = InspectCheckpointFloorRecoveryV068(dir, old)
	if err != nil || state != CheckpointFloorAwaitingExternalV068 {
		t.Fatalf("awaiting expected: %q %v", state, err)
	}
	if _, err = ReadCheckpointFloorStoreV068(dir, old); err == nil {
		t.Fatal("pending state escaped freeze with old external witness")
	}

	state, err = InspectCheckpointFloorRecoveryV068(dir, next)
	if err != nil || state != CheckpointFloorCommitRequiredV068 {
		t.Fatalf("commit-required expected: %q %v", state, err)
	}
	if _, err = ReadCheckpointFloorStoreV068(dir, next); err == nil {
		t.Fatal("pending state escaped freeze with new external witness")
	}

	altered := next
	altered.Digest = strings.Repeat("ff", 32)
	if s, err := InspectCheckpointFloorRecoveryV068(dir, altered); err == nil || s != "" {
		t.Fatalf("unknown witness accepted: %q %v", s, err)
	}
	altered = next
	altered.Revision++
	if _, err := InspectCheckpointFloorRecoveryV068(dir, altered); err == nil {
		t.Fatal("future witness accepted")
	}

	// Inspection must not have advanced or cleared the pending transition.
	if _, err := ReadCheckpointFloorStoreV068(dir, next); err == nil {
		t.Fatal("inspection changed pending state")
	}
	if err := CommitPreparedCheckpointFloorV068(dir, next); err != nil {
		t.Fatal(err)
	}
	state, err = InspectCheckpointFloorRecoveryV068(dir, next)
	if err != nil || state != CheckpointFloorReadyV068 {
		t.Fatalf("post-commit ready expected: %q %v", state, err)
	}
	if _, err := InspectCheckpointFloorRecoveryV068(dir, old); err == nil {
		t.Fatal("old external witness reauthorized after commit")
	}
}

func TestV068RecoveryInspectionFaultBoundaries(t *testing.T) {
	parent, validators, signed, policy, floor, old := v068ForwardCheckpointFixture(t)
	for _, phase := range []string{"before-rename", "after-rename"} {
		t.Run(phase, func(t *testing.T) {
			dir := v068FreshFloorDir(t)
			if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
				t.Fatal(err)
			}
			_, err := prepareCheckpointFloorHookV068(dir, parent, validators, signed, policy, old, func(step string) error {
				if step == phase {
					return errors.New("injected interruption")
				}
				return nil
			})
			if err == nil {
				t.Fatal("fault not propagated")
			}
			state, err := InspectCheckpointFloorRecoveryV068(dir, old)
			want := CheckpointFloorReadyV068
			if phase == "after-rename" {
				want = CheckpointFloorAwaitingExternalV068
			}
			if err != nil || state != want {
				t.Fatalf("unexpected recovered state %q, want %q: %v", state, want, err)
			}
		})
	}
}

func TestV068RecoveryInspectionRejectsAbsentCorruptOrSymlinkedStore(t *testing.T) {
	_, _, _, _, floor, old := v068TrustFloorFixture(t)
	dir := v068FreshFloorDir(t)
	if s, err := InspectCheckpointFloorRecoveryV068(dir, old); err == nil || s != "" {
		t.Fatalf("uninitialized state accepted: %q %v", s, err)
	}
	if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, floorStoreFileV068)
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectCheckpointFloorRecoveryV068(dir, old); err == nil {
		t.Fatal("corrupt record accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectCheckpointFloorRecoveryV068(dir, old); err == nil {
		t.Fatal("symlinked store accepted")
	}
}

// NEGATIVE SECURITY CANARY: an attacker able to restore BOTH the local file
// AND the claimed independent witness can bypass local-only anti-rollback.
// This test intentionally demonstrates the missing external trust root.
func TestV068RecoveryInspectionCannotPreventCoordinatedRollback(t *testing.T) {
	parent, validators, signed, policy, floor, old := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, floorStoreFileV068)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next, err := PrepareCheckpointFloorAdvanceV068(dir, parent, validators, signed, policy, old)
	if err != nil {
		t.Fatal(err)
	}
	if err := CommitPreparedCheckpointFloorV068(dir, next); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectCheckpointFloorRecoveryV068(dir, old); err == nil {
		t.Fatal("old external witness accepted before local rollback")
	}

	// Test-only simulation of compromised external trust and local disk.
	// This is NOT a supported recovery operation or authorized reset.
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	state, err := InspectCheckpointFloorRecoveryV068(dir, old)
	if err != nil || state != CheckpointFloorReadyV068 {
		t.Fatalf("negative canary: coordinated rollback not reproduced: %q %v", state, err)
	}
}
