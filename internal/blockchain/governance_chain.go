package blockchain

import (
	"fmt"

	"prism/internal/reserved"
)

// GetGovernanceState deterministically reconstructs the current
// reserved-authority governance state from the canonical blockchain.
//
// Prism governance has two consensus eras:
//
// Before QueuedGovernanceActivationHeight:
//   - legacy / v0.28 AuthorityChanges are allowed;
//   - AuthorityProposals are forbidden;
//   - AuthorityExecutions are forbidden.
//
// At and after QueuedGovernanceActivationHeight:
//   - direct AuthorityChanges are forbidden;
//   - AuthorityProposals are allowed;
//   - AuthorityExecutions are allowed.
//
// This preserves historical v0.27/v0.28 blocks while preventing direct
// authority changes from bypassing the v0.29 queued-governance lifecycle.
//
// Within a v0.29 block, queued governance is processed in this order:
//
//  1. AuthorityExecutions;
//  2. AuthorityProposals.
//
// Executions therefore mutate the active policy before new proposals in the
// same block are authorized. A proposal included in a block cannot be
// executed by an AuthorityExecution in that same block.
//
// ProposalHeight is consensus-derived from block.Height. ExecuteAfterHeight
// is derived from ProposalHeight + DefaultGovernanceDelayBlocks.
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

		if block.Height < QueuedGovernanceActivationHeight {
			if len(block.AuthorityProposals) != 0 {
				return nil, fmt.Errorf(
					"authority proposals are not allowed before queued governance activation height %d: block height %d",
					QueuedGovernanceActivationHeight,
					block.Height,
				)
			}

			if len(block.AuthorityExecutions) != 0 {
				return nil, fmt.Errorf(
					"authority executions are not allowed before queued governance activation height %d: block height %d",
					QueuedGovernanceActivationHeight,
					block.Height,
				)
			}

			// Historical v0.27/v0.28 direct governance.
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

			continue
		}

		// v0.29 queued-governance era.
		//
		// Direct AuthorityChanges are forbidden from the activation
		// boundary onward. This prevents bypassing the proposal queue,
		// governance delay and explicit execution step.
		if len(block.AuthorityChanges) != 0 {
			return nil, fmt.Errorf(
				"direct authority changes are not allowed at or after queued governance activation height %d: block height %d",
				QueuedGovernanceActivationHeight,
				block.Height,
			)
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

		// New proposals enter the queue only after all executions in this
		// block have completed.
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
