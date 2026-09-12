package blockchain

import (
	"fmt"

	"prism/internal/reserved"
)

// GetGovernanceState deterministically reconstructs the current
// reserved-authority and reserved-transfer governance state from
// the canonical blockchain.
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
	if bc.hasReservedTransferGovernance() {
		governance, _, err :=
			bc.replayReservedTransferConsensusState()

		if err != nil {
			return nil, err
		}

		return governance, nil
	}

	chainID, err :=
		bc.ChainID()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot determine governance chain ID: %w",
			err,
		)
	}

	state, err :=
		reserved.NewGovernanceStateForChain(
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

			if len(block.ReservedTransferProposals) != 0 {
				return nil, fmt.Errorf(
					"genesis block cannot contain reserved transfer proposals",
				)
			}

			if len(block.ReservedTransferExecutions) != 0 {
				return nil, fmt.Errorf(
					"genesis block cannot contain reserved transfer executions",
				)
			}

			continue
		}

		if block.Height <
			QueuedGovernanceActivationHeight {

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

			if len(block.ReservedTransferProposals) != 0 {
				return nil, fmt.Errorf(
					"reserved transfer proposals are not allowed before queued governance activation height %d: block height %d",
					QueuedGovernanceActivationHeight,
					block.Height,
				)
			}

			if len(block.ReservedTransferExecutions) != 0 {
				return nil, fmt.Errorf(
					"reserved transfer executions are not allowed before queued governance activation height %d: block height %d",
					QueuedGovernanceActivationHeight,
					block.Height,
				)
			}

			// Historical v0.27/v0.28 direct governance.
			for changeIndex, change := range block.AuthorityChanges {

				if err :=
					state.ApplyAuthorityChangeAtHeight(
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

		// Queued-governance era.
		if len(block.AuthorityChanges) != 0 {
			return nil, fmt.Errorf(
				"direct authority changes are not allowed at or after queued governance activation height %d: block height %d",
				QueuedGovernanceActivationHeight,
				block.Height,
			)
		}

		// Authority executions mutate the active authority policy first.
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

			if err :=
				state.ExecuteAuthorityProposal(
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

		// Transfer executions need the shared governance + reserved
		// accounting replay introduced by v0.30. Reject them here until
		// that shared replay path handles them atomically.
		if len(block.ReservedTransferExecutions) != 0 {
			return nil, fmt.Errorf(
				"reserved transfer execution consensus replay is not enabled yet",
			)
		}

		// New authority proposals enter the queue after executions.
		for proposalIndex, proposal := range block.AuthorityProposals {

			if err :=
				state.QueueAuthorityProposal(
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

		// Governed reserved transfers use the same consensus-derived
		// proposal height and governance delay.
		for proposalIndex, proposal := range block.ReservedTransferProposals {

			if err :=
				state.QueueReservedTransferProposal(
					proposal,
					chainID,
					block.Height,
					reserved.DefaultGovernanceDelayBlocks,
				); err != nil {

				return nil, fmt.Errorf(
					"invalid reserved transfer proposal in block %d at index %d: %w",
					block.Height,
					proposalIndex,
					err,
				)
			}
		}
	}

	return state, nil
}
