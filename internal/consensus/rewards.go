package consensus

import (
	"fmt"
	"math"
)

const (
	DefaultProposerReward      uint64 = 5
	DefaultUsefulWorkReward    uint64 = 2
	DefaultParticipationReward uint64 = 1
)

type RewardPolicy struct {
	ProposerReward      uint64
	UsefulWorkReward    uint64
	ParticipationReward uint64
}

func DefaultRewardPolicy() RewardPolicy {
	return RewardPolicy{
		ProposerReward:      DefaultProposerReward,
		UsefulWorkReward:    DefaultUsefulWorkReward,
		ParticipationReward: DefaultParticipationReward,
	}
}

func (policy RewardPolicy) Validate() error {
	if policy.ProposerReward == 0 {
		return fmt.Errorf(
			"proposer reward must be greater than zero",
		)
	}

	if policy.UsefulWorkReward == 0 {
		return fmt.Errorf(
			"useful work reward must be greater than zero",
		)
	}

	if policy.ParticipationReward == 0 {
		return fmt.Errorf(
			"participation reward must be greater than zero",
		)
	}

	return nil
}

func (policy RewardPolicy) BlockEmission(
	usefulWorkProofs uint64,
) (uint64, error) {
	return policy.BlockEmissionWithParticipation(
		usefulWorkProofs,
		0,
	)
}

func (policy RewardPolicy) BlockEmissionWithParticipation(
	usefulWorkProofs uint64,
	participationUnits uint64,
) (uint64, error) {
	if err := policy.Validate(); err != nil {
		return 0, err
	}

	if usefulWorkProofs > 0 &&
		policy.UsefulWorkReward >
			math.MaxUint64/usefulWorkProofs {

		return 0, fmt.Errorf(
			"useful work reward overflow",
		)
	}

	workEmission :=
		usefulWorkProofs * policy.UsefulWorkReward

	if participationUnits > 0 &&
		policy.ParticipationReward >
			math.MaxUint64/participationUnits {

		return 0, fmt.Errorf(
			"participation reward overflow",
		)
	}

	participationEmission :=
		participationUnits * policy.ParticipationReward

	if policy.ProposerReward >
		math.MaxUint64-workEmission {

		return 0, fmt.Errorf(
			"block reward emission overflow",
		)
	}

	total :=
		policy.ProposerReward + workEmission

	if total >
		math.MaxUint64-participationEmission {

		return 0, fmt.Errorf(
			"participation emission overflow",
		)
	}

	return total + participationEmission, nil
}
