package participation

import (
	"fmt"
	"math"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

type PeriodReward struct {
	Address string `json:"address"`
	Period  uint64 `json:"period"`
	Points  uint64 `json:"points"`
	Units   uint64 `json:"units"`
	Amount  uint64 `json:"amount"`
}

func boundedParticipationRewardAmount(
	rawAmount uint64,
	emission blockchain.EmissionState,
	supplyPolicy consensus.SupplyPolicy,
) (
	uint64,
	error,
) {
	if err := supplyPolicy.Validate(); err != nil {
		return 0, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	amount, err :=
		consensus.BoundedPoolReward(
			rawAmount,
			emission.ParticipationEmission,
			supplyPolicy.ParticipationRewardPool,
		)

	if err != nil {
		return 0, fmt.Errorf(
			"cannot bound participation reward: %w",
			err,
		)
	}

	return amount, nil
}

func EvaluatePeriodReward(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	eligibility EligibilityChecker,
	address string,
	periodIndex uint64,
) (PeriodReward, error) {
	if chain == nil {
		return PeriodReward{}, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if address == "" {
		return PeriodReward{}, fmt.Errorf(
			"reward address cannot be empty",
		)
	}

	period, err := PeriodByIndex(
		periodIndex,
	)
	if err != nil {
		return PeriodReward{}, fmt.Errorf(
			"invalid reward period: %w",
			err,
		)
	}

	if len(chain.Blocks) == 0 {
		return PeriodReward{}, fmt.Errorf(
			"blockchain has no blocks",
		)
	}

	tipHeight :=
		chain.Blocks[len(chain.Blocks)-1].Height

	if tipHeight < period.EndHeight {
		return PeriodReward{}, fmt.Errorf(
			"reward period %d is not complete: tip=%d end=%d",
			periodIndex,
			tipHeight,
			period.EndHeight,
		)
	}

	scores, err := CalculatePeriod(
		chain,
		pos,
		eligibility,
		periodIndex,
	)
	if err != nil {
		return PeriodReward{}, err
	}

	points := ScoreOf(
		scores,
		address,
	)

	units := RewardUnits(
		points,
	)

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	if err := rewardPolicy.Validate(); err != nil {
		return PeriodReward{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	if units > 0 &&
		rewardPolicy.ParticipationReward >
			math.MaxUint64/units {

		return PeriodReward{}, fmt.Errorf(
			"participation reward amount overflow",
		)
	}

	rawAmount :=
		units * rewardPolicy.ParticipationReward

	emission, err :=
		chain.GetEmissionState()

	if err != nil {
		return PeriodReward{}, fmt.Errorf(
			"cannot calculate participation emissions: %w",
			err,
		)
	}

	amount, err :=
		boundedParticipationRewardAmount(
			rawAmount,
			emission,
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		return PeriodReward{}, err
	}

	return PeriodReward{
		Address: address,
		Period:  periodIndex,
		Points:  points,
		Units:   units,
		Amount:  amount,
	}, nil
}
