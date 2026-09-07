package blockchain

import (
	"fmt"
	"math"

	"prism/internal/consensus"
)

type EmissionState struct {
	ProposerEmission      uint64
	UsefulWorkEmission    uint64
	ParticipationEmission uint64
}

func (state EmissionState) NetworkEmission() (
	uint64,
	error,
) {
	if state.ProposerEmission >
		math.MaxUint64-state.UsefulWorkEmission {

		return 0, fmt.Errorf(
			"network emission overflow",
		)
	}

	total :=
		state.ProposerEmission +
			state.UsefulWorkEmission

	if total >
		math.MaxUint64-state.ParticipationEmission {

		return 0, fmt.Errorf(
			"network emission overflow",
		)
	}

	return total +
			state.ParticipationEmission,
		nil
}

func (bc *Blockchain) GetEmissionState() (
	EmissionState,
	error,
) {
	if bc == nil {
		return EmissionState{}, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	// Validate all consensus-visible state first.
	if _, err := bc.GetState(); err != nil {
		return EmissionState{}, fmt.Errorf(
			"cannot calculate emission state: %w",
			err,
		)
	}

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	if err := rewardPolicy.Validate(); err != nil {
		return EmissionState{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if err := supplyPolicy.Validate(); err != nil {
		return EmissionState{}, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	var emission EmissionState

	for blockIndex, block := range bc.Blocks {

		if blockIndex == 0 {
			continue
		}

		if emission.ProposerEmission >
			math.MaxUint64-block.Reward {

			return EmissionState{}, fmt.Errorf(
				"proposer emission overflow",
			)
		}

		emission.ProposerEmission +=
			block.Reward

		for range block.UsefulWork {
			workReward, err :=
				consensus.BoundedPoolReward(
					rewardPolicy.UsefulWorkReward,
					emission.UsefulWorkEmission,
					supplyPolicy.UsefulWorkRewardPool,
				)

			if err != nil {
				return EmissionState{}, fmt.Errorf(
					"invalid useful work emission: %w",
					err,
				)
			}

			if emission.UsefulWorkEmission >
				math.MaxUint64-workReward {

				return EmissionState{}, fmt.Errorf(
					"useful work emission overflow",
				)
			}

			emission.UsefulWorkEmission +=
				workReward
		}

		for _, claim := range block.ParticipationClaims {

			if emission.ParticipationEmission >
				math.MaxUint64-claim.Amount {

				return EmissionState{}, fmt.Errorf(
					"participation emission overflow",
				)
			}

			emission.ParticipationEmission +=
				claim.Amount
		}
	}

	return emission, nil
}
