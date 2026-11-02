package finality

import (
	"crypto/ed25519"
	"encoding/hex"
	"math"
	"reflect"
	"strings"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/transaction"
)

// Fixtures deliberately use test-only operator and validator keys.
func v068TrustFloorFixture(t *testing.T) (
	*blockchain.Blockchain, []consensus.Validator,
	SignedValidatorCheckpointV068, CheckpointTrustPolicyV068,
	TrustedCheckpointFloorV068, TrustedCheckpointFloorWitnessV068,
) {
	t.Helper()
	bc, validators, signed, policy, _ := v068SignedCheckpointFixture(t)
	floor := TrustedCheckpointFloorV068{Revision: 7, Anchor: signed.Anchor}
	digest, err := CheckpointFloorDigestV068(floor)
	if err != nil {
		t.Fatal(err)
	}
	return bc, validators, signed, policy, floor,
		TrustedCheckpointFloorWitnessV068{Revision: floor.Revision, Digest: digest}
}

func TestV068ExternalFloorIdempotentVerifiedCheckpoint(t *testing.T) {
	bc, validators, signed, policy, floor, witness := v068TrustFloorFixture(t)
	got, seal, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, policy, floor, witness)
	if err != nil {
		t.Fatal(err)
	}
	if got != floor || seal != witness {
		t.Fatal("identical verified checkpoint must not advance trust floor")
	}
}

func v068ForwardCheckpointFixture(t *testing.T) (
	*blockchain.Blockchain, []consensus.Validator,
	SignedValidatorCheckpointV068, CheckpointTrustPolicyV068,
	TrustedCheckpointFloorV068, TrustedCheckpointFloorWitnessV068,
) {
	t.Helper()
	bc, validators, oldSigned, policy, floor, witness := v068TrustFloorFixture(t)
	_, keys, _ := v068Fixture(t)
	pos := &consensus.ProofOfStake{Validators: validators}
	tx := transaction.New(
		validators[0].Address, validators[1].Address, 1, 0,
		hex.EncodeToString(keys[0].Public().(ed25519.PublicKey)),
	)
	if err := tx.Sign(keys[0]); err != nil {
		t.Fatal(err)
	}
	proposer, err := pos.SelectProposer(bc.Blocks[0].Hash, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bc.AddBlock([]transaction.Transaction{tx}, nil, proposer.Address, pos); err != nil {
		t.Fatal(err)
	}
	if !bc.ValidateChain(pos) {
		t.Fatal("new parent must validate")
	}
	nextSigned := oldSigned
	nextSigned.Anchor.ParentHeight = 1
	nextSigned.Anchor.ParentHash = bc.Blocks[1].Hash
	_, _, _, _, key := v068SignedCheckpointFixture(t)
	v068ResignCheckpoint(t, &nextSigned, key)
	return bc, validators, nextSigned, policy, floor, witness
}

func TestV068ExternalFloorAdvancesOnValidatedSignedParent(t *testing.T) {
	bc, validators, nextSigned, policy, floor, witness := v068ForwardCheckpointFixture(t)
	got, seal, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, nextSigned, policy, floor, witness)
	if err != nil {
		t.Fatalf("valid next-height checkpoint rejected: %v", err)
	}
	if got.Revision != 8 || got.Anchor != nextSigned.Anchor || seal.Revision != 8 {
		t.Fatal("monotonic advancement did not preserve exact new anchor")
	}
	expectedDigest, err := CheckpointFloorDigestV068(got)
	if err != nil || seal.Digest != expectedDigest || seal.Digest == witness.Digest {
		t.Fatal("witness did not bind the advanced floor")
	}
	again, againSeal, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, nextSigned, policy, got, seal)
	if err != nil || again != got || againSeal != seal {
		t.Fatalf("new checkpoint retry must be idempotent: %v", err)
	}
	// Note: no caller or disk state was modified by the pure gate.
	if floor.Revision != 7 || witness.Revision != 7 {
		t.Fatal("input mutated")
	}
}

func TestV068ExternalFloorRejectsStaleAndSameHeightConflict(t *testing.T) {
	bc, validators, signed, policy, floor, witness := v068TrustFloorFixture(t)
	elevated := floor
	elevated.Anchor.ParentHeight = 1
	elevated.Anchor.ParentHash = strings.Repeat("aa", 32)
	digest, err := CheckpointFloorDigestV068(elevated)
	if err != nil {
		t.Fatal(err)
	}
	raisedWitness := TrustedCheckpointFloorWitnessV068{Revision: elevated.Revision, Digest: digest}
	if _, _, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, policy, elevated, raisedWitness); err == nil {
		t.Fatal("signed lower-height replay accepted")
	}
	forked := floor
	forked.Anchor.ParentHash = strings.Repeat("bb", 32)
	digest, err = CheckpointFloorDigestV068(forked)
	if err != nil {
		t.Fatal(err)
	}
	forkWitness := TrustedCheckpointFloorWitnessV068{Revision: forked.Revision, Digest: digest}
	if _, _, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, policy, forked, forkWitness); err == nil {
		t.Fatal("same-height conflicting parent accepted")
	}
	forked = floor
	forked.Anchor.ValidatorSetHash = strings.Repeat("cc", 32)
	digest, err = CheckpointFloorDigestV068(forked)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, policy, forked,
		TrustedCheckpointFloorWitnessV068{Revision: forked.Revision, Digest: digest}); err == nil {
		t.Fatal("same-height conflicting validator set accepted")
	}
	// An invalid request must never mutate trusted inputs.
	if !reflect.DeepEqual(floor.Anchor, signed.Anchor) || witness.Revision != 7 {
		t.Fatal("trust floor mutated")
	}
}

func TestV068ExternalFloorRejectsRollbackWitnessAndInvalidSignature(t *testing.T) {
	bc, validators, signed, policy, floor, witness := v068TrustFloorFixture(t)
	tests := []struct {
		name   string
		mutate func(*TrustedCheckpointFloorV068, *TrustedCheckpointFloorWitnessV068)
	}{
		{"old_revision", func(f *TrustedCheckpointFloorV068, _ *TrustedCheckpointFloorWitnessV068) { f.Revision-- }},
		{"wrong_witness_revision", func(_ *TrustedCheckpointFloorV068, w *TrustedCheckpointFloorWitnessV068) { w.Revision-- }},
		{"wrong_witness_digest", func(_ *TrustedCheckpointFloorV068, w *TrustedCheckpointFloorWitnessV068) {
			w.Digest = strings.Repeat("00", 32)
		}},
		{"wrong_floor_network", func(f *TrustedCheckpointFloorV068, _ *TrustedCheckpointFloorWitnessV068) {
			f.Anchor.ChainID = "other-chain"
		}},
		{"wrong_floor_genesis", func(f *TrustedCheckpointFloorV068, _ *TrustedCheckpointFloorWitnessV068) {
			f.Anchor.GenesisHash = strings.Repeat("bb", 32)
		}},
		{"malformed_floor_hash", func(f *TrustedCheckpointFloorV068, _ *TrustedCheckpointFloorWitnessV068) {
			f.Anchor.ParentHash = "not-a-hash"
		}},
		{"zero_revision", func(f *TrustedCheckpointFloorV068, _ *TrustedCheckpointFloorWitnessV068) { f.Revision = 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, w := floor, witness
			tc.mutate(&f, &w)
			if _, _, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, policy, f, w); err == nil {
				t.Fatal("corrupted or rolled-back floor accepted")
			}
		})
	}
	changed := signed
	changed.SignatureHex = strings.Repeat("00", ed25519.SignatureSize)
	if _, _, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, changed, policy, floor, witness); err == nil {
		t.Fatal("invalid signed checkpoint accepted despite matching height")
	}
	revoked := policy
	revoked.Authorities = append([]CheckpointAuthorityV068(nil), policy.Authorities...)
	revoked.Authorities[0].Revoked = true
	if _, _, err := VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, revoked, floor, witness); err == nil {
		t.Fatal("revoked operator accepted despite matching floor")
	}
}

func TestV068ExternalFloorRejectsRevisionOverflow(t *testing.T) {
	bc, validators, signed, policy, floor, _ := v068ForwardCheckpointFixture(t)
	floor.Revision = math.MaxUint64
	digest, err := CheckpointFloorDigestV068(floor)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = VerifyAndAdvanceCheckpointFloorV068(bc, validators, signed, policy, floor,
		TrustedCheckpointFloorWitnessV068{Revision: floor.Revision, Digest: digest})
	if err == nil {
		t.Fatal("higher signed checkpoint caused a revision overflow")
	}
}
