package finality

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

type lockFixtureV065 struct {
	validators []consensus.Validator
	keys       []ed25519.PrivateKey
	setHash    string
	genesis    string
	blockA     string
	blockB     string
}

func hashLabV065(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func fixtureLockV065(t *testing.T) lockFixtureV065 {
	t.Helper()
	f := lockFixtureV065{
		genesis: hashLabV065("locking-genesis"),
		blockA:  hashLabV065("escrow-block-a"),
		blockB:  hashLabV065("escrow-block-b"),
	}
	for i, stake := range []uint64{4, 3, 2, 1} {
		seed := make([]byte, ed25519.SeedSize)
		for j := range seed {
			seed[j] = byte(1 + i)
		}
		key := ed25519.NewKeyFromSeed(seed)
		f.keys = append(f.keys, key)
		f.validators = append(f.validators, consensus.Validator{
			Address: wallet.AddressFromPublicKey(key.Public().(ed25519.PublicKey)),
			Stake:   stake,
		})
	}
	var err error
	f.setHash, err = ValidatorSetHash(f.validators)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f lockFixtureV065) machine(t *testing.T) *BFTLockMachineV065 {
	t.Helper()
	m, err := NewBFTLockMachineV065("prism-lock-lab", f.genesis, 3, f.validators)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (f lockFixtureV065) certificate(t *testing.T, step, block string, round uint64, signers ...int) Certificate {
	t.Helper()
	ctx := VoteContext{
		ChainID: "prism-lock-lab", GenesisHash: f.genesis,
		Height: 3, Round: round, Step: step,
		BlockHash: block, ValidatorSetHash: f.setHash,
	}
	cert := Certificate{Context: ctx}
	for _, index := range signers {
		v, err := SignVote(ctx, f.keys[index])
		if err != nil {
			t.Fatal(err)
		}
		cert.Votes = append(cert.Votes, v)
	}
	return cert
}

func mustChooseLockV065(t *testing.T, m *BFTLockMachineV065, proposal string, valid bool, pol *Certificate, expected string) {
	t.Helper()
	got, err := m.ChoosePrevoteV065(proposal, valid, pol)
	if err != nil || got != expected {
		t.Fatalf("prevote choice=%q err=%v expected=%q", got, err, expected)
	}
}

func mustPrecommitLockV065(t *testing.T, m *BFTLockMachineV065, pol *Certificate, want string) {
	t.Helper()
	got, err := m.EnterPrecommitV065(pol, pol != nil)
	if err != nil || got != want {
		t.Fatalf("precommit choice=%q err=%v expected=%q", got, err, want)
	}
}

func TestBFTLockV065UncommittedTransitions(t *testing.T) {
	f := fixtureLockV065(t)
	m := f.machine(t)
	if _, err := NewBFTLockMachineV065("", f.genesis, 3, f.validators); err == nil {
		t.Fatal("accepted missing network")
	}
	if _, err := NewBFTLockMachineV065("prism-lock-lab", f.genesis, 0, f.validators); err == nil {
		t.Fatal("accepted height zero")
	}
	original := m.SnapshotV065()
	if _, err := m.ChoosePrevoteV065("not-a-hash", true, nil); err == nil {
		t.Fatal("accepted noncanonical block")
	}
	if m.SnapshotV065() != original {
		t.Fatal("invalid proposal mutated state")
	}
	mustChooseLockV065(t, m, "", false, nil, "")
	if _, err := m.ChoosePrevoteV065(f.blockA, true, nil); err == nil {
		t.Fatal("allowed second prevote decision in same round")
	}
	mustPrecommitLockV065(t, m, nil, "")
	if m.SnapshotV065().HasLock {
		t.Fatal("NIL recommendation unexpectedly created lock")
	}
	if err := m.AdvanceRoundV065(0); err == nil {
		t.Fatal("allowed nonincreasing round")
	}
	if err := m.AdvanceRoundV065(1); err != nil {
		t.Fatal(err)
	}
	mustChooseLockV065(t, m, f.blockA, true, nil, f.blockA)
	proof := f.certificate(t, stepPrevote, f.blockA, 1, 0, 1)
	mustPrecommitLockV065(t, m, &proof, f.blockA)
	s := m.SnapshotV065()
	if !s.HasLock || s.LockedRound != 1 || s.LockedBlock != f.blockA ||
		!s.HasValid || s.ValidBlock != f.blockA || s.ValidRound != 1 {
		t.Fatalf("expected round-one lock and valid block: %+v", s)
	}
	if _, err := m.EnterPrecommitV065(&proof, true); err == nil {
		t.Fatal("allowed second precommit decision")
	}
}

func TestBFTLockV065CrossRoundConflictRequiresVerifiedPoLC(t *testing.T) {
	f := fixtureLockV065(t)
	m := f.machine(t)
	mustChooseLockV065(t, m, f.blockA, true, nil, f.blockA)
	quorumA := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	mustPrecommitLockV065(t, m, &quorumA, f.blockA)
	if err := m.AdvanceRoundV065(1); err != nil {
		t.Fatal(err)
	}
	// Without evidence for B, the prior A lock determines the PREVOTE.
	mustChooseLockV065(t, m, f.blockB, true, nil, f.blockA)
	mustPrecommitLockV065(t, m, nil, "")
	if s := m.SnapshotV065(); !s.HasLock || s.LockedBlock != f.blockA {
		t.Fatal("PRECOMMIT(NIL) simulation erased existing lock")
	}
	if err := m.AdvanceRoundV065(2); err != nil {
		t.Fatal(err)
	}
	original := m.SnapshotV065()

	good := f.certificate(t, stepPrevote, f.blockB, 1, 0, 1)
	underweight := f.certificate(t, stepPrevote, f.blockB, 1, 0, 2) // only 6/10
	stale := f.certificate(t, stepPrevote, f.blockB, 0, 0, 1)
	future := f.certificate(t, stepPrevote, f.blockB, 2, 0, 1)
	other := f.certificate(t, stepPrevote, f.blockA, 1, 0, 1)
	wrongStep := f.certificate(t, stepPrecommit, f.blockB, 1, 0, 1)
	wrongHeight := good
	wrongHeight.Context.Height++
	wrongChain := good
	wrongChain.Context.ChainID = "other-prism-network"
	wrongSnapshot := good
	wrongSnapshot.Context.ValidatorSetHash = hashLabV065("fake-set")
	invalidSignature := good
	invalidSignature.Votes = append([]SignedVote(nil), good.Votes...)
	invalidSignature.Votes[0].Signature = strings.Repeat("0", ed25519.SignatureSize*2)
	duplicate := good
	duplicate.Votes = append(append([]SignedVote(nil), good.Votes...), good.Votes[0])

	bad := []struct {
		name string
		cert Certificate
	}{
		{"insufficient_stake", underweight},
		{"stale_round", stale},
		{"current_round", future},
		{"wrong_block", other},
		{"wrong_step", wrongStep},
		{"wrong_height", wrongHeight},
		{"wrong_network", wrongChain},
		{"wrong_validator_snapshot", wrongSnapshot},
		{"forged_signature", invalidSignature},
		{"duplicate_signature", duplicate},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := m.ChoosePrevoteV065(f.blockB, true, &tc.cert); err == nil {
				t.Fatal("accepted invalid PoLC")
			}
			if m.SnapshotV065() != original {
				t.Fatal("invalid PoLC mutated lock state")
			}
		})
	}
	mustChooseLockV065(t, m, f.blockB, true, &good, f.blockB)
	if m.SnapshotV065().HasLock {
		t.Fatal("verified higher-round PoLC did not release A lock")
	}
	quorumB := f.certificate(t, stepPrevote, f.blockB, 2, 0, 1)
	mustPrecommitLockV065(t, m, &quorumB, f.blockB)
	if s := m.SnapshotV065(); !s.HasLock || s.LockedBlock != f.blockB || s.LockedRound != 2 {
		t.Fatalf("expected new lock on B at round 2: %+v", s)
	}
}

func TestBFTLockV065PrecommitAndCommitCertificateChecks(t *testing.T) {
	f := fixtureLockV065(t)
	m := f.machine(t)
	mustChooseLockV065(t, m, f.blockA, true, nil, f.blockA)
	initial := m.SnapshotV065()
	underweight := f.certificate(t, stepPrevote, f.blockA, 0, 0, 2)
	future := f.certificate(t, stepPrevote, f.blockA, 1, 0, 1)
	for _, cert := range []Certificate{underweight, future} {
		if _, err := m.EnterPrecommitV065(&cert, true); err == nil {
			t.Fatal("accepted invalid prevote evidence")
		}
		if m.SnapshotV065() != initial {
			t.Fatal("invalid precommit evidence mutated machine")
		}
	}
	validPrevotes := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	if _, err := m.EnterPrecommitV065(&validPrevotes, false); err == nil {
		t.Fatal("locked unvalidated or unavailable block")
	}
	if m.SnapshotV065() != initial {
		t.Fatal("unvalidated block mutated lock state")
	}
	mustPrecommitLockV065(t, m, &validPrevotes, f.blockA)
	commitA := f.certificate(t, stepPrecommit, f.blockA, 0, 0, 1)
	if err := m.ObserveCommitV065(commitA, false); err == nil {
		t.Fatal("committed unavailable or invalid block")
	}
	invalidCommit := f.certificate(t, stepPrecommit, f.blockA, 0, 0, 2)
	if err := m.ObserveCommitV065(invalidCommit, true); err == nil {
		t.Fatal("accepted underweight precommit certificate")
	}
	if m.SnapshotV065().Finalized {
		t.Fatal("invalid commit finalized state")
	}
	if err := m.ObserveCommitV065(commitA, true); err != nil {
		t.Fatal(err)
	}
	stable := m.SnapshotV065()
	if stable.Step != BFTCommitV065 || !stable.Finalized || stable.FinalizedBlock != f.blockA {
		t.Fatalf("expected lab COMMIT state: %+v", stable)
	}
	if err := m.ObserveCommitV065(commitA, true); err != nil {
		t.Fatalf("idempotent commit rejected: %v", err)
	}
	commitB := f.certificate(t, stepPrecommit, f.blockB, 0, 0, 1)
	if err := m.ObserveCommitV065(commitB, true); err == nil {
		t.Fatal("accepted contradictory COMMIT at same height")
	}
	if err := m.AdvanceRoundV065(1); err == nil {
		t.Fatal("advanced after COMMIT")
	}
	if _, err := m.ChoosePrevoteV065(f.blockB, true, nil); err == nil {
		t.Fatal("prevoted after COMMIT")
	}
	if !reflect.DeepEqual(stable, m.SnapshotV065()) {
		t.Fatal("post-COMMIT invalid actions mutated state")
	}
}

func TestBFTLockV065TrustedValidatorSnapshotCloned(t *testing.T) {
	f := fixtureLockV065(t)
	m := f.machine(t)
	f.validators[0].Stake = 0
	mustChooseLockV065(t, m, f.blockA, true, nil, f.blockA)
	cert := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	mustPrecommitLockV065(t, m, &cert, f.blockA)
	if !m.SnapshotV065().HasLock {
		t.Fatal("mutable caller stake array corrupted the trusted snapshot")
	}
}

func TestBFTLockV065NotDistributedFinality(t *testing.T) {
	f := fixtureLockV065(t)
	// Without anti-equivocation integrated for actual validators, two
	// independent lab machines can still observe contradictory valid certificates.
	a := f.machine(t)
	b := f.machine(t)
	certA := f.certificate(t, stepPrecommit, f.blockA, 0, 0, 1)
	certB := f.certificate(t, stepPrecommit, f.blockB, 0, 0, 1)
	if err := a.ObserveCommitV065(certA, true); err != nil {
		t.Fatal(err)
	}
	if err := b.ObserveCommitV065(certB, true); err != nil {
		t.Fatal(err)
	}
	if a.SnapshotV065().FinalizedBlock == b.SnapshotV065().FinalizedBlock {
		t.Fatal("fixture did not demonstrate conflicting independent certificates")
	}
	t.Log("LIMITATION: standalone signed quorum observation is not a BFT safety proof")
}
