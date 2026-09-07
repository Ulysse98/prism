package consensus

import (
	"fmt"
	"math"
)

const (
	DefaultProposerReward   uint64 = 5
	DefaultUsefulWorkReward uint64 = 2
)

type RewardPolicy struct {
	ProposerReward   uint64
	UsefulWorkReward uint64
}

func DefaultRewardPolicy() RewardPolicy {
	return RewardPolicy{
		ProposerReward:   DefaultProposerReward,
		UsefulWorkReward: DefaultUsefulWorkReward,
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

	return nil
}

func (policy RewardPolicy) BlockEmission(
	usefulWorkProofs uint64,
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

	if policy.ProposerReward >
		math.MaxUint64-workEmission {

		return 0, fmt.Errorf(
			"block reward emission overflow",
		)
	}

	return policy.ProposerReward + workEmission, nil
}
