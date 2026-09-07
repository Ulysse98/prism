package blockchain

import (
	"fmt"
	"math"

	"prism/internal/consensus"
	"prism/internal/poup"
)

type participationClaimKey struct {
	Address string
	Period  uint64
}

func (bc *Blockchain) expectedParticipationReward(
	address string,
	periodIndex uint64,
	policy consensus.RewardPolicy,
) (
	uint64,
	uint64,
	uint64,
	error,
) {
	if bc == nil {
		return 0, 0, 0, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if address == "" {
		return 0, 0, 0, fmt.Errorf(
			"participation reward address cannot be empty",
		)
	}

	if err := policy.Validate(); err != nil {
		return 0, 0, 0, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	period, err := poup.PeriodByIndex(
		periodIndex,
	)
	if err != nil {
		return 0, 0, 0, fmt.Errorf(
			"invalid participation reward period: %w",
			err,
		)
	}

	var points uint64

	for blockIndex, block := range bc.Blocks {
		if blockIndex == 0 {
			continue
		}

		if block.Height < period.StartHeight ||
			block.Height > period.EndHeight {

			continue
		}

		if block.Proposer == address &&
			bc.IsVerifiedAtHeight(
				address,
				block.Height,
			) {

			if points >
				math.MaxUint64-poup.ProposerPoints {

				return 0, 0, 0, fmt.Errorf(
					"participation proposer points overflow",
				)
			}

			points += poup.ProposerPoints
		}

		for _, proof := range block.UsefulWork {
			if proof.Worker != address {
				continue
			}

			if !bc.IsVerifiedAtHeight(
				proof.Worker,
				block.Height,
			) {
				continue
			}

			if proof.Score >
				math.MaxUint64/
					poup.UsefulWorkPointMultiple {

				return 0, 0, 0, fmt.Errorf(
					"participation useful work points overflow",
				)
			}

			contribution :=
				proof.Score *
					poup.UsefulWorkPointMultiple

			if points >
				math.MaxUint64-contribution {

				return 0, 0, 0, fmt.Errorf(
					"participation points overflow",
				)
			}

			points += contribution
		}
	}

	units := poup.RewardUnits(
		points,
	)

	if units > 0 &&
		policy.ParticipationReward >
			math.MaxUint64/units {

		return 0, 0, 0, fmt.Errorf(
			"participation reward amount overflow",
		)
	}

	amount :=
		units * policy.ParticipationReward

	return points, units, amount, nil
}

func (bc *Blockchain) validateParticipationClaim(
	claim poup.Claim,
	claimHeight uint64,
	policy consensus.RewardPolicy,
) error {
	if err := poup.ValidateSigned(
		claim,
	); err != nil {
		return fmt.Errorf(
			"invalid signed participation claim: %w",
			err,
		)
	}

	period, err := poup.PeriodByIndex(
		claim.Period,
	)
	if err != nil {
		return fmt.Errorf(
			"invalid participation claim period: %w",
			err,
		)
	}

	if claimHeight <= period.EndHeight {
		return fmt.Errorf(
			"participation claim period %d is not complete at height %d",
			claim.Period,
			claimHeight,
		)
	}

	expectedPoints,
		expectedUnits,
		expectedAmount,
		err :=
		bc.expectedParticipationReward(
			claim.Address,
			claim.Period,
			policy,
		)

	if err != nil {
		return err
	}

	if claim.Points != expectedPoints {
		return fmt.Errorf(
			"invalid participation claim points: expected %d, got %d",
			expectedPoints,
			claim.Points,
		)
	}

	if claim.Units != expectedUnits {
		return fmt.Errorf(
			"invalid participation claim units: expected %d, got %d",
			expectedUnits,
			claim.Units,
		)
	}

	if claim.Amount != expectedAmount {
		return fmt.Errorf(
			"invalid participation claim amount: expected %d, got %d",
			expectedAmount,
			claim.Amount,
		)
	}

	return nil
}
