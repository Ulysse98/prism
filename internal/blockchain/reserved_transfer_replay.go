package blockchain

import (
	"fmt"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

// hasReservedTransferGovernance reports whether the canonical chain contains
// at least one governed reserved-transfer execution.
//
// Chains without transfer executions retain the exact historical v0.29
// governance/accounting replay paths.
func (bc *Blockchain) hasReservedTransferGovernance() bool {
	if bc == nil {
		return false
	}

	for _, block := range bc.Blocks {
		if len(block.ReservedTransferProposals) != 0 ||
			len(block.ReservedTransferExecutions) != 0 {

			return true
		}
	}

	return false
}

// replayReservedTransferConsensusState deterministically reconstructs
// governance and reserved accounting together.
//
// Legacy reserved operations are evaluated against the authority policy active
// at the beginning of the block.
//
// In the queued-governance era the order is:
//
//  1. legacy reserved operations;
//  2. authority executions;
//  3. reserved-transfer executions;
//  4. authority proposals;
//  5. reserved-transfer proposals.
//
// This lets an authority revocation executed in a block invalidate a governed
// transfer execution later in that same block.
func (bc *Blockchain) replayReservedTransferConsensusState() (
	*reserved.GovernanceState,
	*reserved.AccountingState,
	error,
) {
	if bc == nil {
		return nil, nil,
			fmt.Errorf(
				"blockchain cannot be nil",
			)
	}

	if len(bc.Blocks) == 0 {
		return nil, nil,
			fmt.Errorf(
				"blockchain has no genesis block",
			)
	}

	chainID, err :=
		bc.ChainID()

	if err != nil {
		return nil, nil,
			fmt.Errorf(
				"cannot determine reserved consensus chain ID: %w",
				err,
			)
	}

	genesisSupply, err :=
		bc.GenesisSupply()

	if err != nil {
		return nil, nil,
			fmt.Errorf(
				"cannot determine genesis supply: %w",
				err,
			)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err :=
		supplyPolicy.Validate(); err != nil {

		return nil, nil,
			fmt.Errorf(
				"invalid supply policy: %w",
				err,
			)
	}

	governance, err :=
		reserved.NewGovernanceStateForChain(
			chainID,
			bc.Config.ReservedAuthorities,
		)

	if err != nil {
		return nil, nil,
			fmt.Errorf(
				"cannot initialize reserved governance state: %w",
				err,
			)
	}

	accounting :=
		reserved.NewAccountingState(
			genesisSupply,
		)

	for blockIndex, block := range bc.Blocks {

		if blockIndex == 0 {
			if len(block.ReservedAuthorizations) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain reserved authorizations",
					)
			}

			if len(block.ReservedGrants) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain reserved grants",
					)
			}

			if len(block.ReservedRevocations) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain reserved revocations",
					)
			}

			if len(block.AuthorityChanges) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain reserved authority changes",
					)
			}

			if len(block.AuthorityProposals) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain authority proposals",
					)
			}

			if len(block.AuthorityExecutions) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain authority executions",
					)
			}

			if len(block.ReservedTransferProposals) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain reserved transfer proposals",
					)
			}

			if len(block.ReservedTransferExecutions) != 0 {
				return nil, nil,
					fmt.Errorf(
						"genesis block cannot contain reserved transfer executions",
					)
			}

			continue
		}

		queuedEra :=
			block.Height >=
				QueuedGovernanceActivationHeight

		transferEra :=
			block.Height >=
				GovernedReservedTransferActivationHeight

		if !queuedEra {
			if len(block.AuthorityProposals) != 0 {
				return nil, nil,
					fmt.Errorf(
						"authority proposals are not allowed before queued governance activation height %d: block height %d",
						QueuedGovernanceActivationHeight,
						block.Height,
					)
			}

			if len(block.AuthorityExecutions) != 0 {
				return nil, nil,
					fmt.Errorf(
						"authority executions are not allowed before queued governance activation height %d: block height %d",
						QueuedGovernanceActivationHeight,
						block.Height,
					)
			}
		} else if len(block.AuthorityChanges) != 0 {
			return nil, nil,
				fmt.Errorf(
					"direct authority changes are not allowed at or after queued governance activation height %d: block height %d",
					QueuedGovernanceActivationHeight,
					block.Height,
				)
		}

		if !transferEra &&
			(len(block.ReservedTransferProposals) != 0 ||
				len(block.ReservedTransferExecutions) != 0) {

			return nil, nil,
				fmt.Errorf(
					"reserved transfer governance is not allowed before governed transfer activation height %d: block height %d",
					GovernedReservedTransferActivationHeight,
					block.Height,
				)
		}

		if transferEra &&
			len(block.ReservedGrants) != 0 {

			return nil, nil,
				fmt.Errorf(
					"legacy reserved grants are not allowed at or after governed transfer activation height %d: block height %d",
					GovernedReservedTransferActivationHeight,
					block.Height,
				)
		}

		// Legacy reserved operations use the authority policy that was active
		// at the beginning of this block.
		authorityPolicy :=
			governance.CurrentPolicy

		for authorizationIndex, authorization := range block.ReservedAuthorizations {

			if err :=
				accounting.Accept(
					authorization,
					authorityPolicy,
					chainID,
					supplyPolicy,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid reserved authorization in block %d at index %d: %w",
						block.Height,
						authorizationIndex,
						err,
					)
			}
		}

		for revocationIndex, revocation := range block.ReservedRevocations {

			if err :=
				accounting.AcceptRevocation(
					revocation,
					authorityPolicy,
					chainID,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid reserved revocation in block %d at index %d: %w",
						block.Height,
						revocationIndex,
						err,
					)
			}
		}

		for grantIndex, grant := range block.ReservedGrants {

			if err :=
				accounting.AcceptGrantAtHeight(
					grant,
					block.Height,
					authorityPolicy,
					chainID,
					supplyPolicy,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid reserved grant in block %d at index %d: %w",
						block.Height,
						grantIndex,
						err,
					)
			}
		}

		if !queuedEra {
			for changeIndex, change := range block.AuthorityChanges {

				if err :=
					governance.ApplyAuthorityChangeAtHeight(
						change,
						chainID,
						block.Height,
					); err != nil {

					return nil, nil,
						fmt.Errorf(
							"invalid reserved authority change in block %d at index %d: %w",
							block.Height,
							changeIndex,
							err,
						)
				}
			}

			continue
		}

		// v0.29 authority executions happen before v0.30 transfer
		// executions, so an authority removed here cannot authorize
		// a transfer later in this same block.
		for executionIndex, execution := range block.AuthorityExecutions {

			if err :=
				reserved.ValidateAuthorityExecution(
					execution,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid authority execution in block %d at index %d: %w",
						block.Height,
						executionIndex,
						err,
					)
			}

			if err :=
				governance.ExecuteAuthorityProposal(
					execution.ProposalID,
					chainID,
					block.Height,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"authority proposal execution failed in block %d at index %d: %w",
						block.Height,
						executionIndex,
						err,
					)
			}
		}

		for executionIndex, execution := range block.ReservedTransferExecutions {

			if err :=
				reserved.ValidateReservedTransferExecution(
					execution,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid reserved transfer execution in block %d at index %d: %w",
						block.Height,
						executionIndex,
						err,
					)
			}

			if _, err :=
				governance.ExecuteReservedTransferProposal(
					execution.ProposalID,
					chainID,
					block.Height,
					accounting,
					supplyPolicy,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"reserved transfer execution failed in block %d at index %d: %w",
						block.Height,
						executionIndex,
						err,
					)
			}
		}

		for proposalIndex, proposal := range block.AuthorityProposals {

			if err :=
				governance.QueueAuthorityProposal(
					proposal,
					chainID,
					block.Height,
					reserved.DefaultGovernanceDelayBlocks,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid authority proposal in block %d at index %d: %w",
						block.Height,
						proposalIndex,
						err,
					)
			}
		}

		for proposalIndex, proposal := range block.ReservedTransferProposals {
			if err :=
				accounting.ValidateReservedTransferQueueReplay(
					proposal,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"reserved transfer proposal replay rejected in block %d at index %d: %w",
						block.Height,
						proposalIndex,
						err,
					)
			}

			if err :=
				governance.QueueReservedTransferProposal(
					proposal,
					chainID,
					block.Height,
					reserved.DefaultGovernanceDelayBlocks,
				); err != nil {

				return nil, nil,
					fmt.Errorf(
						"invalid reserved transfer proposal in block %d at index %d: %w",
						block.Height,
						proposalIndex,
						err,
					)
			}
		}
	}

	return governance,
		accounting,
		nil
}
