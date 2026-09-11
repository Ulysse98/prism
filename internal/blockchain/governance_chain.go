package blockchain

import (
	"fmt"

	"prism/internal/reserved"
)

// GetGovernanceState deterministically reconstructs the current
// reserved-authority governance state from the canonical blockchain.
//
// Governance processing order inside each block is:
//
//  1. direct AuthorityChanges;
//  2. queued AuthorityExecutions;
//  3. new AuthorityProposals.
//
// Executions therefore mutate the active policy before new proposals in the
// same block are authorized. A proposal included in a block can never be
// executed by an AuthorityExecution in that same block.
//
// Legacy AuthorityChanges remain supported. Timelocked v0.28 changes are
// applied only once the block carrying them has reached ActivationHeight.
//
// v0.29 AuthorityProposals derive their ProposalHeight from block.Height and
// use DefaultGovernanceDelayBlocks as their consensus delay.
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

			if len(block.AuthorityProposals) != 0 {
				return nil, fmt.Errorf(
					"genesis block cannot contain authority proposals",
				)
			}

			if len(block.AuthorityExecutions) != 0 {
				return nil, fmt.Errorf(
					"genesis block cannot contain authority executions",
				)
			}

			continue
		}

		// Legacy / v0.28 direct governance is applied first.
		for changeIndex, change := range block.AuthorityChanges {
			if err := state.ApplyAuthorityChangeAtHeight(
				change,
				chainID,
				block.Height,
			); err != nil {
				return nil, fmt.Errorf(
					"invalid reserved authority change in block %d at index %d: %w",
					block.Height,
					changeIndex,
					err,
				)
			}
		}

		// Execute proposals that were queued in earlier blocks.
		for executionIndex, execution := range block.AuthorityExecutions {

			if err :=
				reserved.ValidateAuthorityExecution(
					execution,
				); err != nil {

				return nil, fmt.Errorf(
					"invalid authority execution in block %d at index %d: %w",
					block.Height,
					executionIndex,
					err,
				)
			}

			if err := state.ExecuteAuthorityProposal(
				execution.ProposalID,
				chainID,
				block.Height,
			); err != nil {
				return nil, fmt.Errorf(
					"authority proposal execution failed in block %d at index %d: %w",
					block.Height,
					executionIndex,
					err,
				)
			}
		}

		// New proposals enter the queue only after all governance mutations
		// scheduled for this block have completed.
		for proposalIndex, proposal := range block.AuthorityProposals {

			if err := state.QueueAuthorityProposal(
				proposal,
				chainID,
				block.Height,
				reserved.DefaultGovernanceDelayBlocks,
			); err != nil {
				return nil, fmt.Errorf(
					"invalid authority proposal in block %d at index %d: %w",
					block.Height,
					proposalIndex,
					err,
				)
			}
		}
	}

	return state, nil
}
