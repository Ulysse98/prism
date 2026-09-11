package blockchain

import (
	"fmt"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

// AddAuthorityProposalBlock creates a Prism block containing queued
// authority-governance proposals.
//
// The proposal inclusion height is consensus-derived from the block height.
// The complete candidate chain is validated before the local blockchain
// is mutated.
func (bc *Blockchain) AddAuthorityProposalBlock(
	proposals []reserved.AuthorityProposal,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if len(proposals) == 0 {
		return Block{}, fmt.Errorf(
			"authority proposal block cannot be empty",
		)
	}

	blockProposals :=
		make(
			[]reserved.AuthorityProposal,
			len(proposals),
		)

	for index, proposal := range proposals {
		if err :=
			reserved.ValidateAuthorityProposal(
				proposal,
			); err != nil {

			return Block{}, fmt.Errorf(
				"invalid authority proposal at index %d: %w",
				index,
				err,
			)
		}

		// AuthorityProposal contains an AuthorityChange whose approvals
		// are a slice. Copy them so later caller mutation cannot alter
		// an already-produced block.
		cloned :=
			proposal

		cloned.Change.Approvals =
			append(
				[]reserved.Approval(nil),
				proposal.Change.Approvals...,
			)

		blockProposals[index] =
			cloned
	}

	return bc.addQueuedGovernanceBlock(
		blockProposals,
		nil,
		proposer,
		pos,
	)
}

// AddAuthorityExecutionBlock creates a Prism block containing queued
// authority-governance executions.
//
// Execution height is never supplied by the caller. The canonical
// execution height is the containing block's consensus height.
func (bc *Blockchain) AddAuthorityExecutionBlock(
	executions []reserved.AuthorityExecution,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if len(executions) == 0 {
		return Block{}, fmt.Errorf(
			"authority execution block cannot be empty",
		)
	}

	blockExecutions :=
		make(
			[]reserved.AuthorityExecution,
			len(executions),
		)

	for index, execution := range executions {
		if err :=
			reserved.ValidateAuthorityExecution(
				execution,
			); err != nil {

			return Block{}, fmt.Errorf(
				"invalid authority execution at index %d: %w",
				index,
				err,
			)
		}

		blockExecutions[index] =
			execution
	}

	return bc.addQueuedGovernanceBlock(
		nil,
		blockExecutions,
		proposer,
		pos,
	)
}

func (bc *Blockchain) addQueuedGovernanceBlock(
	proposals []reserved.AuthorityProposal,
	executions []reserved.AuthorityExecution,
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
			"queued governance block cannot be empty",
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

	if err := bc.ValidateValidatorSet(
		pos,
	); err != nil {
		return Block{}, err
	}

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	if nextHeight <
		QueuedGovernanceActivationHeight {

		return Block{}, fmt.Errorf(
			"queued governance is not active until height %d: next block height %d",
			QueuedGovernanceActivationHeight,
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

	if proposer != expectedProposer.Address {
		return Block{}, fmt.Errorf(
			"invalid proposer for height %d: expected %s, got %s",
			nextHeight,
			expectedProposer.Address,
			proposer,
		)
	}

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	if err := rewardPolicy.Validate(); err != nil {
		return Block{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err := supplyPolicy.Validate(); err != nil {
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
		Height:              nextHeight,
		Timestamp:           time.Now().UTC(),
		PreviousHash:        previous.Hash,
		Proposer:            proposer,
		Reward:              proposerReward,
		AuthorityProposals:  proposals,
		AuthorityExecutions: executions,
	}

	block.Hash =
		CalculateHash(block)

	// Use the exact received-block validation path for locally produced
	// queued-governance blocks. AppendValidatedBlock validates the full
	// candidate chain before mutating bc.Blocks.
	if err := bc.AppendValidatedBlock(
		block,
		pos,
	); err != nil {

		return Block{}, fmt.Errorf(
			"invalid queued governance block: %w",
			err,
		)
	}

	return block, nil
}
