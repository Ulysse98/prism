package blockchain

import (
	"fmt"
	"math"

	"prism/internal/consensus"
)

type RewardPoolState struct {
	ProposerRemaining      uint64
	UsefulWorkRemaining    uint64
	ParticipationRemaining uint64
}

func (state EmissionState) RemainingRewardPools(
	policy consensus.SupplyPolicy,
) (
	RewardPoolState,
	error,
) {
	if err := policy.Validate(); err != nil {
		return RewardPoolState{}, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	if state.ProposerEmission >
		policy.ProposerRewardPool {

		return RewardPoolState{}, fmt.Errorf(
			"proposer emission exceeds reward pool",
		)
	}

	if state.UsefulWorkEmission >
		policy.UsefulWorkRewardPool {

		return RewardPoolState{}, fmt.Errorf(
			"useful work emission exceeds reward pool",
		)
	}

	if state.ParticipationEmission >
		policy.ParticipationRewardPool {

		return RewardPoolState{}, fmt.Errorf(
			"participation emission exceeds reward pool",
		)
	}

	return RewardPoolState{
		ProposerRemaining: policy.ProposerRewardPool -
			state.ProposerEmission,

		UsefulWorkRemaining: policy.UsefulWorkRewardPool -
			state.UsefulWorkEmission,

		ParticipationRemaining: policy.ParticipationRewardPool -
			state.ParticipationEmission,
	}, nil
}

func (state RewardPoolState) NetworkRemaining() (
	uint64,
	error,
) {
	if state.ProposerRemaining >
		math.MaxUint64-state.UsefulWorkRemaining {

		return 0, fmt.Errorf(
			"remaining network reward pool overflow",
		)
	}

	total :=
		state.ProposerRemaining +
			state.UsefulWorkRemaining

	if total >
		math.MaxUint64-state.ParticipationRemaining {

		return 0, fmt.Errorf(
			"remaining network reward pool overflow",
		)
	}

	return total +
			state.ParticipationRemaining,
		nil
}
