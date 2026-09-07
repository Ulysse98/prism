package blockchain

import (
	"fmt"
	"time"

	"prism/internal/consensus"
	"prism/internal/poup"
)

// AddParticipationClaimBlock creates a Prism block containing
// signed Proof of Useful Participation reward claims.
//
// The complete candidate chain is validated before the local
// blockchain is mutated.
func (bc *Blockchain) AddParticipationClaimBlock(
	claims []poup.Claim,
	proposer string,
	pos *consensus.ProofOfStake,
) (Block, error) {
	if bc == nil {
		return Block{}, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if len(claims) == 0 {
		return Block{}, fmt.Errorf(
			"participation claim block cannot be empty",
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

	if err := bc.ValidateValidatorSet(
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

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

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

	blockClaims :=
		append(
			[]poup.Claim(nil),
			claims...,
		)

	block := Block{
		Height:              nextHeight,
		Timestamp:           time.Now().UTC(),
		PreviousHash:        previous.Hash,
		Proposer:            proposer,
		Reward:              proposerReward,
		ParticipationClaims: blockClaims,
	}

	block.Hash =
		CalculateHash(block)

	// Validate the complete candidate chain before mutating bc.
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
			"invalid participation claim block",
		)
	}

	bc.Blocks =
		append(
			bc.Blocks,
			block,
		)

	return block, nil
}
