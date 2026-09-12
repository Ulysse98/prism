package blockchain

import (
	"fmt"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func cloneReservedTransferProposals(
	proposals []reserved.ReservedTransferProposal,
) []reserved.ReservedTransferProposal {
	if len(proposals) == 0 {
		return nil
	}

	cloned :=
		make(
			[]reserved.ReservedTransferProposal,
			len(proposals),
		)

	for index, proposal := range proposals {
		copyProposal :=
			proposal

		copyProposal.Approvals =
			append(
				[]reserved.Approval(nil),
				proposal.Approvals...,
			)

		cloned[index] =
			copyProposal
	}

	return cloned
}

func cloneReservedTransferExecutions(
	executions []reserved.ReservedTransferExecution,
) []reserved.ReservedTransferExecution {
	if len(executions) == 0 {
		return nil
	}

	return append(
		[]reserved.ReservedTransferExecution(nil),
		executions...,
	)
}

// AddReservedTransferProposalBlock creates a v0.30 governed-reserved-transfer
// proposal block.
//
// Proposal inclusion height is derived exclusively from the containing block.
// The complete candidate chain is validated before bc.Blocks is mutated.
func (bc *Blockchain) AddReservedTransferProposalBlock(
	proposals []reserved.ReservedTransferProposal,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if len(proposals) == 0 {
		return Block{}, fmt.Errorf(
			"reserved transfer proposal block cannot be empty",
		)
	}

	return bc.addReservedTransferGovernanceBlock(
		cloneReservedTransferProposals(
			proposals,
		),
		nil,
		proposer,
		pos,
	)
}

// AddReservedTransferExecutionBlock creates a v0.30 governed-reserved-transfer
// execution block.
//
// The execution height is the containing block's consensus height. Callers
// supply only proposal IDs; timelock and active-policy checks are performed
// by canonical consensus replay.
func (bc *Blockchain) AddReservedTransferExecutionBlock(
	executions []reserved.ReservedTransferExecution,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if len(executions) == 0 {
		return Block{}, fmt.Errorf(
			"reserved transfer execution block cannot be empty",
		)
	}

	blockExecutions :=
		cloneReservedTransferExecutions(
			executions,
		)

	for index, execution := range blockExecutions {

		if err :=
			reserved.ValidateReservedTransferExecution(
				execution,
			); err != nil {

			return Block{}, fmt.Errorf(
				"invalid reserved transfer execution at index %d: %w",
				index,
				err,
			)
		}
	}

	return bc.addReservedTransferGovernanceBlock(
		nil,
		blockExecutions,
		proposer,
		pos,
	)
}

func (bc *Blockchain) addReservedTransferGovernanceBlock(
	proposals []reserved.ReservedTransferProposal,
	executions []reserved.ReservedTransferExecution,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if bc == nil {
		return Block{}, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if len(proposals) == 0 &&
		len(executions) == 0 {

		return Block{}, fmt.Errorf(
			"reserved transfer governance block cannot be empty",
		)
	}

	if pos == nil {
		return Block{}, fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if proposer == "" {
		return Block{}, fmt.Errorf(
			"block proposer cannot be empty",
		)
	}

	if proposer == "GENESIS" {
		return Block{}, fmt.Errorf(
			"GENESIS cannot propose normal blocks",
		)
	}

	if len(bc.Blocks) == 0 {
		return Block{}, fmt.Errorf(
			"blockchain has no genesis block",
		)
	}

	if err :=
		bc.ValidateValidatorSet(
			pos,
		); err != nil {

		return Block{}, err
	}

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	if nextHeight <
		GovernedReservedTransferActivationHeight {

		return Block{}, fmt.Errorf(
			"governed reserved transfers are not active until height %d: next block height %d",
			GovernedReservedTransferActivationHeight,
			nextHeight,
		)
	}

	expectedProposer, err :=
		pos.SelectProposer(
			previous.Hash,
			nextHeight,
		)

	if err != nil {
		return Block{}, err
	}

	if proposer !=
		expectedProposer.Address {

		return Block{}, fmt.Errorf(
			"invalid proposer for height %d: expected %s, got %s",
			nextHeight,
			expectedProposer.Address,
			proposer,
		)
	}

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	if err :=
		rewardPolicy.Validate(); err != nil {

		return Block{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err :=
		supplyPolicy.Validate(); err != nil {

		return Block{}, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	emission, err :=
		bc.GetEmissionState()

	if err != nil {
		return Block{}, fmt.Errorf(
			"cannot calculate current emissions: %w",
			err,
		)
	}

	proposerReward, err :=
		consensus.BoundedPoolReward(
			rewardPolicy.ProposerReward,
			emission.ProposerEmission,
			supplyPolicy.ProposerRewardPool,
		)

	if err != nil {
		return Block{}, fmt.Errorf(
			"cannot calculate proposer reward: %w",
			err,
		)
	}

	block := Block{
		Height:       nextHeight,
		Timestamp:    time.Now().UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer,
		Reward:       proposerReward,
		ReservedTransferProposals: cloneReservedTransferProposals(
			proposals,
		),
		ReservedTransferExecutions: cloneReservedTransferExecutions(
			executions,
		),
	}

	block.Hash =
		CalculateHash(block)

	if err :=
		bc.AppendValidatedBlock(
			block,
			pos,
		); err != nil {

		return Block{}, fmt.Errorf(
			"invalid governed reserved transfer block: %w",
			err,
		)
	}

	// Return a separate deep copy so caller mutation cannot alter slices
	// potentially retained by the canonical block.
	block.ReservedTransferProposals =
		cloneReservedTransferProposals(
			block.ReservedTransferProposals,
		)

	block.ReservedTransferExecutions =
		cloneReservedTransferExecutions(
			block.ReservedTransferExecutions,
		)

	return block, nil
}
