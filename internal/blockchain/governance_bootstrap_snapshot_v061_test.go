package blockchain

import (
	"reflect"
	"testing"
)

func v061PinnedSnapshotFixture(t *testing.T) *Blockchain {
	t.Helper()
	chain := &Blockchain{
		Blocks:       make([]Block, v061CheckpointHeight+1),
		LockedStakes: map[string]uint64{},
	}
	chain.Blocks[0].Hash = v061CheckpointGenesisHash
	chain.Blocks[v061CheckpointHeight].Height = v061CheckpointHeight
	chain.Blocks[v061CheckpointHeight].Hash = v061CheckpointBlockHash
	return chain
}

func TestV061HistoricalBootstrapStakeSnapshotDoesNotDrift(t *testing.T) {
	bc := v061PinnedSnapshotFixture(t)
	original, err := bc.pinnedBootstrapValidatorsForChainV061()
	if err != nil {
		t.Fatal(err)
	}
	bc.LockedStakes[original[0].Address] = 999999
	bc.LockedStakes["prism_fake_later_validator"] = 1000000
	after, err := bc.pinnedBootstrapValidatorsForChainV061()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, after) {
		t.Fatal("later LockedStakes rewrote the checkpoint voting weights")
	}
}

func TestV061HistoricalBootstrapSnapshotRejectsWrongCheckpoint(t *testing.T) {
	bc := v061PinnedSnapshotFixture(t)
	bc.Blocks[v061CheckpointHeight].Hash = "wrong"
	if _, err := bc.pinnedBootstrapValidatorsForChainV061(); err == nil {
		t.Fatal("unrecognized block 5 was accepted")
	}
	bc = v061PinnedSnapshotFixture(t)
	bc.Blocks[0].Hash = "wrong"
	if _, err := bc.pinnedBootstrapValidatorsForChainV061(); err == nil {
		t.Fatal("unrecognized genesis was accepted")
	}
	bc = v061PinnedSnapshotFixture(t)
	bc.Blocks = bc.Blocks[:5]
	if _, err := bc.pinnedBootstrapValidatorsForChainV061(); err == nil {
		t.Fatal("missing checkpoint block was accepted")
	}
}
