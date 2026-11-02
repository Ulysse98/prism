package finality

import (
	"strings"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// The pin is constructed from this fixture only because these are tests.
// A real verifier MUST obtain its pin from an independent trusted source.
func v068PinnedSnapshotFixture(t *testing.T) (*blockchain.Blockchain, []consensus.Validator, PinnedValidatorAnchorV068) {
	t.Helper()
	validators, _, setHash := v068Fixture(t)
	balances := make(map[string]uint64, len(validators))
	for _, v := range validators {
		balances[v.Address] = v.Stake + 100
	}
	bc, err := blockchain.NewBlockchain(balances)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range validators {
		if err := bc.LockStake(v.Address, v.Stake); err != nil {
			t.Fatal(err)
		}
	}
	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	if !bc.ValidateChain(&consensus.ProofOfStake{Validators: validators}) {
		t.Fatal("fixture parent chain not valid")
	}
	tip := bc.Blocks[len(bc.Blocks)-1]
	pin := PinnedValidatorAnchorV068{
		ChainID:          chainID,
		GenesisHash:      bc.Blocks[0].Hash,
		ParentHeight:     tip.Height,
		ParentHash:       tip.Hash,
		ValidatorSetHash: setHash,
	}
	return bc, validators, pin
}

func TestV068PinnedValidatorSnapshotAcceptsExactExternallyPinnedContext(t *testing.T) {
	bc, validators, pin := v068PinnedSnapshotFixture(t)
	if err := VerifyPinnedValidatorSnapshotV068(bc, validators, pin); err != nil {
		t.Fatal(err)
	}
	reordered := append([]consensus.Validator(nil), validators...)
	reordered[0], reordered[2] = reordered[2], reordered[0]
	if err := VerifyPinnedValidatorSnapshotV068(bc, reordered, pin); err != nil {
		t.Fatalf("valid reordered set rejected: %v", err)
	}
}

func TestV068PinnedValidatorSnapshotRejectsWrongAnchorFields(t *testing.T) {
	bc, validators, pin := v068PinnedSnapshotFixture(t)
	tests := []struct {
		name   string
		mutate func(*PinnedValidatorAnchorV068)
	}{
		{"missing_chain_id", func(p *PinnedValidatorAnchorV068) { p.ChainID = "" }},
		{"wrong_chain_id", func(p *PinnedValidatorAnchorV068) { p.ChainID = "prism-attacker" }},
		{"wrong_genesis", func(p *PinnedValidatorAnchorV068) { p.GenesisHash = strings.Repeat("ab", 32) }},
		{"wrong_parent_height", func(p *PinnedValidatorAnchorV068) { p.ParentHeight++ }},
		{"wrong_parent_hash", func(p *PinnedValidatorAnchorV068) { p.ParentHash = strings.Repeat("cd", 32) }},
		{"wrong_validator_set_hash", func(p *PinnedValidatorAnchorV068) { p.ValidatorSetHash = strings.Repeat("ef", 32) }},
		{"noncanonical_hash", func(p *PinnedValidatorAnchorV068) { p.ParentHash = strings.ToUpper(p.ParentHash) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			altered := pin
			tc.mutate(&altered)
			if err := VerifyPinnedValidatorSnapshotV068(bc, validators, altered); err == nil {
				t.Fatal("untrusted or mismatched anchor accepted")
			}
		})
	}
}

func TestV068PinnedValidatorSnapshotRejectsForgedStakeAndValidatorSet(t *testing.T) {
	bc, validators, pin := v068PinnedSnapshotFixture(t)
	changed := append([]consensus.Validator(nil), validators...)
	changed[0].Stake++
	if err := VerifyPinnedValidatorSnapshotV068(bc, changed, pin); err == nil {
		t.Fatal("changed stake accepted")
	}
	duplicate := append(append([]consensus.Validator(nil), validators...), validators[0])
	if err := VerifyPinnedValidatorSnapshotV068(bc, duplicate, pin); err == nil {
		t.Fatal("duplicate validator accepted")
	}
	if err := VerifyPinnedValidatorSnapshotV068(bc, nil, pin); err == nil {
		t.Fatal("empty validator set accepted")
	}
	if err := VerifyPinnedValidatorSnapshotV068(nil, validators, pin); err == nil {
		t.Fatal("nil parent accepted")
	}

	// Even a matching forged set commitment must fail if the actual parent
	// chain's locked stake does not match it.
	forged := append([]consensus.Validator(nil), validators...)
	forged[0].Stake++
	forgedHash, err := ValidatorSetHash(forged)
	if err != nil {
		t.Fatal(err)
	}
	fakePin := pin
	fakePin.ValidatorSetHash = forgedHash
	if err := VerifyPinnedValidatorSnapshotV068(bc, forged, fakePin); err == nil {
		t.Fatal("self-consistent but parent-incompatible stake accepted")
	}
}

func TestV068PinnedValidatorSnapshotRejectsChangedParentState(t *testing.T) {
	bc, validators, pin := v068PinnedSnapshotFixture(t)
	victim := validators[0].Address

	before := bc.LockedStakes[victim]
	bc.LockedStakes[victim] = before + 1
	if err := VerifyPinnedValidatorSnapshotV068(bc, validators, pin); err == nil {
		t.Fatal("modified parent locked stake accepted")
	}
	bc.LockedStakes[victim] = before

	// Leave the committed hash in place but mutate the genesis body.
	bc.Blocks[0].Timestamp = bc.Blocks[0].Timestamp.Add(time.Second)
	if err := VerifyPinnedValidatorSnapshotV068(bc, validators, pin); err == nil {
		t.Fatal("tampered genesis body accepted")
	}
}
