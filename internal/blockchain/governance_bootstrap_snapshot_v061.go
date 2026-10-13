package blockchain

import (
	"fmt"

	"prism/internal/consensus"
)

// pinnedBootstrapValidatorsForChainV061 derives a stable validator snapshot
// from the explicitly coordinated v0.61 checkpoint, NOT from current stake.
// Caller must perform full block hash and chain validation separately.
func (bc *Blockchain) pinnedBootstrapValidatorsForChainV061() ([]consensus.Validator, error) {
	if bc == nil || len(bc.Blocks) <= int(v061CheckpointHeight) {
		return nil, fmt.Errorf("v0.61 pinned checkpoint is absent")
	}
	block5 := bc.Blocks[v061CheckpointHeight]
	if block5.Height != v061CheckpointHeight {
		return nil, fmt.Errorf("v0.61 checkpoint block height mismatch")
	}
	id, err := bc.ChainID()
	if err != nil {
		return nil, fmt.Errorf("v0.61 chain ID: %w", err)
	}
	return pinnedCheckpointValidatorsV061(id, bc.Blocks[0].Hash, block5.Hash)
}
