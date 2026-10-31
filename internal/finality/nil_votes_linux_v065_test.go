//go:build linux

package finality

import (
	"crypto/ed25519"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func nilLabCertificateV065(t *testing.T, f lockFixtureV065, step string, round uint64, signerIDs ...int) Certificate {
	t.Helper()
	c := VoteContext{
		ChainID: "prism-lock-lab", GenesisHash: f.genesis, Height: 3,
		Round: round, Step: step, Nil: true, ValidatorSetHash: f.setHash,
	}
	cert := Certificate{Context: c}
	for _, i := range signerIDs {
		vote, err := SignVote(c, f.keys[i])
		if err != nil {
			t.Fatal(err)
		}
		cert.Votes = append(cert.Votes, vote)
	}
	return cert
}

func TestNilV065ExplicitEncodingQuorumAndLegacyCompatibility(t *testing.T) {
	f := fixtureLockV065(t)
	nilCert := nilLabCertificateV065(t, f, stepPrevote, 0, 0, 1)
	if signed, total, err := VerifyQuorum(f.validators, nilCert); err != nil || signed != 7 || total != 10 {
		t.Fatalf("NIL prevote quorum: signed=%d total=%d err=%v", signed, total, err)
	}
	small := nilLabCertificateV065(t, f, stepPrevote, 0, 0, 2)
	if _, _, err := VerifyQuorum(f.validators, small); err == nil {
		t.Fatal("underweight NIL quorum accepted")
	}
	precommit := nilLabCertificateV065(t, f, stepPrecommit, 0, 0, 1)
	if _, _, err := VerifyQuorum(f.validators, precommit); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		context VoteContext
	}{
		{"nil_with_nonempty_hash", func() VoteContext { c := nilCert.Context; c.BlockHash = f.blockA; return c }()},
		{"non_nil_with_empty_hash", func() VoteContext { c := nilCert.Context; c.Nil = false; return c }()},
		{"nil_with_malformed_hash", func() VoteContext { c := nilCert.Context; c.BlockHash = "NIL"; return c }()},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SignVote(tt.context, f.keys[0]); err == nil {
				t.Fatal("signed ambiguous choice")
			}
		})
	}
	mixed := nilCert
	mixed.Votes = append([]SignedVote(nil), nilCert.Votes...)
	altered := mixed.Votes[0]
	altered.Context.Nil = false
	altered.Context.BlockHash = f.blockA
	mixed.Votes[0] = altered
	if _, _, err := VerifyQuorum(f.validators, mixed); err == nil {
		t.Fatal("accepted mixed vote choices")
	}
	tampered := nilCert
	tampered.Votes = append([]SignedVote(nil), nilCert.Votes...)
	tampered.Votes[0].Signature = strings.Repeat("0", ed25519.SignatureSize*2)
	if _, _, err := VerifyQuorum(f.validators, tampered); err == nil {
		t.Fatal("accepted forged NIL signature")
	}

	// Existing v0.65 non-NIL sign-bytes must remain unchanged. NIL uses omitempty
	// and a new explicit true bit; the lab domain stays the same for block votes.
	old := journalTestContextV065()
	signed, err := SignVote(old, journalTestKeyV065())
	if err != nil {
		t.Fatal(err)
	}
	const legacySignature = "31afcadf5be0ce5bae3713f91f6dfec72297e5553a0d287049950e60101af5d34db551dbd30a894bb580a736216ac3bfa0c3e7a61334bb9a821d207b2a69a500"
	if signed.Signature != legacySignature {
		t.Fatalf("legacy signing bytes changed: %s", signed.Signature)
	}
}

func TestNilV065JournalRecoveryAndSlotEquivocation(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	ctx := journalTestContextV065()
	ctx.BlockHash = ""
	ctx.Nil = true
	key := journalTestKeyV065()
	first, err := SignVoteOnceV065(dir, ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := SignVoteOnceV065(dir, ctx, key)
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatalf("NIL retry failed: %v", err)
	}
	stored, err := readVoteJournalV065(filepath.Join(dir, voteJournalFilenameV065))
	if err != nil || len(stored.Votes) != 1 || stored.Votes[0] != first {
		t.Fatalf("stored NIL vote invalid: %v", err)
	}
	other := ctx
	other.Nil = false
	other.BlockHash = strings.Repeat("a", 64)
	if _, err := SignVoteOnceV065(dir, other, key); err == nil {
		t.Fatal("signed block after NIL in same slot")
	}
	alt := ctx
	alt.ValidatorSetHash = strings.Repeat("d", 64)
	if _, err := SignVoteOnceV065(dir, alt, key); err == nil {
		t.Fatal("signed NIL under modified snapshot in same slot")
	}
	dir2 := privateVoteTestDirV065(t)
	if _, err := SignVoteOnceV065(dir2, other, key); err != nil {
		t.Fatal(err)
	}
	if _, err := SignVoteOnceV065(dir2, ctx, key); err == nil {
		t.Fatal("signed NIL after block in same slot")
	}
}

func TestNilV065MachineUnlocksOnlyOnVerifiedNilQuorum(t *testing.T) {
	f := fixtureLockV065(t)
	m := f.machine(t)
	mustChooseLockV065(t, m, f.blockA, true, nil, f.blockA)
	certA := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	mustPrecommitLockV065(t, m, &certA, f.blockA)
	if err := m.AdvanceRoundV065(1); err != nil {
		t.Fatal(err)
	}
	mustChooseLockV065(t, m, f.blockB, true, nil, f.blockA)
	locked := m.SnapshotV065()
	nilCert := nilLabCertificateV065(t, f, stepPrevote, 1, 0, 1)
	underweight := nilLabCertificateV065(t, f, stepPrevote, 1, 0, 2)
	wrongRound := nilLabCertificateV065(t, f, stepPrevote, 0, 0, 1)
	wrongStep := nilLabCertificateV065(t, f, stepPrecommit, 1, 0, 1)
	for _, tc := range []struct {
		name      string
		cert      Certificate
		validated bool
	}{
		{"underweight", underweight, false},
		{"wrong_round", wrongRound, false},
		{"wrong_step", wrongStep, false},
		{"nil_claims_validated_block", nilCert, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := m.EnterPrecommitV065(&tc.cert, tc.validated); err == nil {
				t.Fatal("invalid NIL unlock accepted")
			}
			if m.SnapshotV065() != locked {
				t.Fatal("rejected NIL proof mutated state")
			}
		})
	}
	choice, err := m.EnterPrecommitV065(&nilCert, false)
	if err != nil || choice != "" {
		t.Fatalf("NIL quorum did not precommit NIL: %q %v", choice, err)
	}
	s := m.SnapshotV065()
	if s.HasLock || s.LockedBlock != "" || s.LockedRound != 0 || s.Step != BFTPrecommitV065 || s.PrecommitChoice != "" {
		t.Fatalf("NIL quorum failed to unlock: %+v", s)
	}
	if err := m.AdvanceRoundV065(2); err != nil {
		t.Fatal(err)
	}
	mustChooseLockV065(t, m, f.blockB, true, nil, f.blockB)
	certB := f.certificate(t, stepPrevote, f.blockB, 2, 0, 1)
	mustPrecommitLockV065(t, m, &certB, f.blockB)
	if m.SnapshotV065().LockedBlock != f.blockB {
		t.Fatal("failed to relock B after justified NIL unlock")
	}
}

func TestNilV065NilPrecommitCannotFinalize(t *testing.T) {
	f := fixtureLockV065(t)
	m := f.machine(t)
	cert := nilLabCertificateV065(t, f, stepPrecommit, 0, 0, 1)
	before := m.SnapshotV065()
	if err := m.ObserveCommitV065(cert, true); err == nil {
		t.Fatal("NIL cert finalized a block")
	}
	if m.SnapshotV065() != before {
		t.Fatal("NIL commit mutated state")
	}
}
