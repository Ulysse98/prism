package blockchain

import (
	"fmt"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func cloneReservedRevocations(
	revocations []reserved.Revocation,
) []reserved.Revocation {
	if len(revocations) == 0 {
		return nil
	}

	cloned := make(
		[]reserved.Revocation,
		len(revocations),
	)

	for index, revocation := range revocations {
		cloned[index] = revocation

		cloned[index].Approvals =
			append(
				[]reserved.Approval(nil),
				revocation.Approvals...,
			)
	}

	return cloned
}

// AddReservedRevocationBlock creates a Prism block containing
// threshold-authorized reserved grant revocations.
//
// The complete candidate chain is validated before the local
// blockchain is mutated.
func (bc *Blockchain) AddReservedRevocationBlock(
	revocations []reserved.Revocation,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if bc == nil {
		return Block{}, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if len(revocations) == 0 {
		return Block{}, fmt.Errorf(
			"reserved revocation block cannot be empty",
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

	if err :=
		bc.ValidateValidatorSet(
			pos,
		); err != nil {

		return Block{}, err
	}

	if len(bc.Blocks) == 0 {
		return Block{}, fmt.Errorf(
			"blockchain has no genesis block",
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

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	for index, revocation := range revocations {

		if err :=
			reserved.ValidateRevocation(
				revocation,
			); err != nil {

			return Block{}, fmt.Errorf(
				"reserved revocation at index %d is invalid: %w",
				index,
				err,
			)
		}
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
		ReservedRevocations: cloneReservedRevocations(
			revocations,
		),
	}

	block.Hash =
		CalculateHash(block)

	candidate := *bc

	candidate.Blocks =
		append(
			[]Block(nil),
			bc.Blocks...,
		)

	candidate.Blocks =
		append(
			candidate.Blocks,
			block,
		)

	if !candidate.ValidateChain(pos) {
		return Block{}, fmt.Errorf(
			"invalid reserved revocation block",
		)
	}

	storedBlock := block

	storedBlock.ReservedRevocations =
		cloneReservedRevocations(
			block.ReservedRevocations,
		)

	bc.Blocks =
		append(
			bc.Blocks,
			storedBlock,
		)

	return block, nil
}
