package crosschain

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

func newSnapshotOperatorV067(t *testing.T, f anchorIncludedFixtureV067, cp PinnedSourceCheckpointV067) *LocalOperatorAdmitterV067 {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedOperatorTrustV067{
		Policy: f.candidate.policy, Checkpoint: cp,
		Signature: ed25519.Sign(private, message),
	}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	operator, err := NewLocalOperatorAdmitterV067(store, public, signed, f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	return operator
}

func TestLocalOperatorSnapshotV067DetachesNestedState(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	operator := newSnapshotOperatorV067(t, f, cp)

	if operator.chain == f.chain || operator.pos == f.pos {
		t.Fatal("snapshot aliases source top-level pointers")
	}
	if &operator.chain.Blocks[1].UsefulWork[0] == &f.chain.Blocks[1].UsefulWork[0] {
		t.Fatal("snapshot aliases nested proof")
	}
	if &operator.pos.Validators[0] == &f.pos.Validators[0] {
		t.Fatal("snapshot aliases validator slice")
	}

	// Mutations AFTER snapshot construction must not invalidate operator's
	// source view or alter the signed policy/checkpoint captured by value.
	f.chain.Blocks[1].UsefulWork[0].Signature = "00"
	f.chain.Blocks[1].Hash = strings.Repeat("f", 64)
	f.chain.Blocks = nil
	f.pos.Validators[0].Stake = 0
	f.pos.Validators = nil

	c := f.candidate
	_, err := operator.Admit(LocalAnchorCandidateV067{
		Intent: c.intent, Job: c.job, Contract: c.contract,
		Proof: c.proof, Report: c.report,
	})
	if err != nil {
		t.Fatalf("captured valid source rejected after original changed: %v", err)
	}
	if _, err := operator.Admit(LocalAnchorCandidateV067{
		Intent: c.intent, Job: c.job, Contract: c.contract,
		Proof: c.proof, Report: c.report,
	}); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("expected replay rejection against captured snapshot, got %v", err)
	}
}

func TestLocalOperatorSnapshotV067RejectsCorruptSourceAtInitialization(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	f.chain.Blocks[1].Hash = strings.Repeat("f", 64)

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedOperatorTrustV067{
		Policy: f.candidate.policy, Checkpoint: cp,
		Signature: ed25519.Sign(private, message),
	}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalOperatorAdmitterV067(store, public, signed, f.chain, f.pos); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("expected invalid source rejection, got %v", err)
	}
}

func TestLocalOperatorSnapshotV067RejectsMissingAndEmptySource(t *testing.T) {
	f, _ := newCheckpointFixtureV067(t)
	if _, _, err := snapshotOperatorSourceV067(nil, f.pos); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("missing chain accepted: %v", err)
	}
	if _, _, err := snapshotOperatorSourceV067(f.chain, nil); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("missing validator set accepted: %v", err)
	}
	f.pos.Validators = nil
	if _, _, err := snapshotOperatorSourceV067(f.chain, f.pos); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("empty validator set accepted: %v", err)
	}
}
