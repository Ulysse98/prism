package blockchain

import (
	"fmt"

	"prism/internal/reserved"
)

// GetGovernanceState deterministically reconstructs the current
// reserved-authority governance state from the canonical blockchain.
//
// The configured authority policy is the genesis governance policy.
// Every AuthorityChange recorded after genesis is replayed in block
// order and change order.
func (bc *Blockchain) GetGovernanceState() (
	*reserved.GovernanceState,
	error,
) {
	if bc == nil {
		return nil, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if len(bc.Blocks) == 0 {
		return nil, fmt.Errorf(
			"blockchain has no genesis block",
		)
	}

	chainID, err := bc.ChainID()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot determine governance chain ID: %w",
			err,
		)
	}

	state, err := reserved.NewGovernanceStateForChain(
		chainID,
		bc.Config.ReservedAuthorities,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot initialize blockchain governance state: %w",
			err,
		)
	}

	for blockIndex, block := range bc.Blocks {
		if blockIndex == 0 {
			if len(block.AuthorityChanges) != 0 {
				return nil, fmt.Errorf(
					"genesis block cannot contain reserved authority changes",
				)
			}

			continue
		}

		for changeIndex, change := range block.AuthorityChanges {
			if err := state.ApplyAuthorityChange(
				change,
				chainID,
			); err != nil {
				return nil, fmt.Errorf(
					"invalid reserved authority change in block %d at index %d: %w",
					block.Height,
					changeIndex,
					err,
				)
			}
		}
	}

	return state, nil
}
