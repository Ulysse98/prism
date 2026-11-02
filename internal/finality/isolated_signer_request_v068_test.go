package finality

import (
	"math"
	"strings"
	"testing"

	"prism/internal/consensus"
)

func v068IsolatedSignerRequestFixture(t *testing.T) (
	IsolatedSignerRequestV068, TrustedCheckpointFloorV068,
	TrustedCheckpointFloorWitnessV068, []consensus.Validator,
) {
	t.Helper()
	_, validators, _, _, floor, witness := v068TrustFloorFixture(t)
	req := IsolatedSignerRequestV068{
		Vote: VoteContext{
			ChainID:          floor.Anchor.ChainID,
			GenesisHash:      floor.Anchor.GenesisHash,
			Height:           floor.Anchor.ParentHeight + 1,
			Round:            0,
			Step:             stepPrevote,
			BlockHash:        strings.Repeat("cd", 32),
			ValidatorSetHash: floor.Anchor.ValidatorSetHash,
		},
		ParentHash:    floor.Anchor.ParentHash,
		SignerAddress: validators[0].Address,
	}
	return req, floor, witness, validators
}

func TestV068SignerRequestContractAcceptsExactRestrictedContext(t *testing.T) {
	req, floor, witness, validators := v068IsolatedSignerRequestFixture(t)
	if err := CheckIsolatedSignerRequestV068(req, floor, witness, validators); err != nil {
		t.Fatal(err)
	}
	reordered := append([]consensus.Validator(nil), validators...)
	reordered[0], reordered[2] = reordered[2], reordered[0]
	if err := CheckIsolatedSignerRequestV068(req, floor, witness, reordered); err != nil {
		t.Fatalf("canonical snapshot order should not matter: %v", err)
	}
}

func TestV068SignerRequestContractRejectsWrongOrUnsupportedContext(t *testing.T) {
	req, floor, witness, validators := v068IsolatedSignerRequestFixture(t)
	variants := []struct {
		name   string
		change func(*IsolatedSignerRequestV068)
	}{
		{"chain", func(x *IsolatedSignerRequestV068) { x.Vote.ChainID = "other-chain" }},
		{"genesis", func(x *IsolatedSignerRequestV068) { x.Vote.GenesisHash = strings.Repeat("cc", 32) }},
		{"height", func(x *IsolatedSignerRequestV068) { x.Vote.Height++ }},
		{"parent", func(x *IsolatedSignerRequestV068) { x.ParentHash = strings.Repeat("dd", 32) }},
		{"set", func(x *IsolatedSignerRequestV068) { x.Vote.ValidatorSetHash = strings.Repeat("ee", 32) }},
		{"round", func(x *IsolatedSignerRequestV068) { x.Vote.Round = 1 }},
		{"precommit", func(x *IsolatedSignerRequestV068) { x.Vote.Step = stepPrecommit }},
		{"nil", func(x *IsolatedSignerRequestV068) { x.Vote.Nil = true; x.Vote.BlockHash = "" }},
		{"unknown_signer", func(x *IsolatedSignerRequestV068) { x.SignerAddress = "unknown" }},
		{"missing_parent", func(x *IsolatedSignerRequestV068) { x.ParentHash = "" }},
		{"bad_block_hash", func(x *IsolatedSignerRequestV068) { x.Vote.BlockHash = "not-a-hash" }},
	}
	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			changed := req
			tc.change(&changed)
			if err := CheckIsolatedSignerRequestV068(changed, floor, witness, validators); err == nil {
				t.Fatal("incorrect or unsupported request passed contract")
			}
		})
	}
}

func TestV068SignerRequestContractRejectsWrongTrustedInputs(t *testing.T) {
	req, floor, witness, validators := v068IsolatedSignerRequestFixture(t)
	stale := witness
	stale.Revision--
	if err := CheckIsolatedSignerRequestV068(req, floor, stale, validators); err == nil {
		t.Fatal("stale witness accepted")
	}
	modified := append([]consensus.Validator(nil), validators...)
	modified[0].Stake++
	if err := CheckIsolatedSignerRequestV068(req, floor, witness, modified); err == nil {
		t.Fatal("incorrect stake accepted")
	}
	if err := CheckIsolatedSignerRequestV068(req, floor, witness, nil); err == nil {
		t.Fatal("missing validators accepted")
	}
	overflow := floor
	overflow.Anchor.ParentHeight = math.MaxUint64
	digest, err := CheckpointFloorDigestV068(overflow)
	if err != nil {
		t.Fatal(err)
	}
	newWitness := TrustedCheckpointFloorWitnessV068{Revision: overflow.Revision, Digest: digest}
	if err := CheckIsolatedSignerRequestV068(req, overflow, newWitness, validators); err == nil {
		t.Fatal("height overflow accepted")
	}
}

// NEGATIVE SECURITY CANARY: two conflicting requests for the SAME signing
// slot both pass a pure input check. Phase 2A.2+ requires one isolated key
// owner and a durable, global anti-equivocation gate BEFORE signing.
func TestV068SignerRequestContractDoesNotPreventEquivocation(t *testing.T) {
	req, floor, witness, validators := v068IsolatedSignerRequestFixture(t)
	conflicting := req
	conflicting.Vote.BlockHash = strings.Repeat("ef", 32)
	if err := CheckIsolatedSignerRequestV068(req, floor, witness, validators); err != nil {
		t.Fatal(err)
	}
	if err := CheckIsolatedSignerRequestV068(conflicting, floor, witness, validators); err != nil {
		t.Fatal(err)
	}
	t.Log("EXPECTED LIMITATION: independent requests for conflicting blocks pass the pure contract")
}
