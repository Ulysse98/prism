package blockchain

import (
	"reflect"
	"testing"
)

func TestV061PinnedCheckpointMatchesRecordedManifest(t *testing.T) {
	got, err := pinnedCheckpointValidatorsV061(
		v061CheckpointChainID, v061CheckpointGenesisHash, v061CheckpointBlockHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("validators=%d, want 3", len(got))
	}
	var total uint64
	for _, v := range got {
		total += v.Stake
	}
	if total != 1000 {
		t.Fatalf("total stake=%d, want 1000", total)
	}

	// There is no reference to Blockchain.LockedStakes in the resolver.
	// Changing a caller's returned list cannot alter the next resolution.
	original := append([]byte(nil), []byte(got[0].Address)...)
	got[0].Stake = 999999
	got[0].Address = "prism_attacker"
	again, err := pinnedCheckpointValidatorsV061(
		v061CheckpointChainID, v061CheckpointGenesisHash, v061CheckpointBlockHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Stake != 300 || !reflect.DeepEqual([]byte(again[0].Address), original) {
		t.Fatal("v0.61 checkpoint validator manifest was mutated")
	}
}

func TestV061PinnedCheckpointRejectsWrongIdentity(t *testing.T) {
	cases := []struct {
		name                  string
		chain, genesis, block string
	}{
		{"wrong chain ID", "prism-wrong", v061CheckpointGenesisHash, v061CheckpointBlockHash},
		{"wrong genesis", v061CheckpointChainID, "not-the-genesis", v061CheckpointBlockHash},
		{"wrong checkpoint block", v061CheckpointChainID, v061CheckpointGenesisHash, "not-the-block"},
		{"empty identity", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vals, err := pinnedCheckpointValidatorsV061(tc.chain, tc.genesis, tc.block)
			if err == nil || vals != nil {
				t.Fatal("untrusted checkpoint was accepted")
			}
		})
	}
}
