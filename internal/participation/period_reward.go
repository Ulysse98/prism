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

	policy := consensus.DefaultRewardPolicy()

	if err := policy.Validate(); err != nil {
		return PeriodReward{}, fmt.Errorf(
			"invalid reward policy: %w",
			err,
		)
	}

	if units > 0 &&
		policy.ParticipationReward >
			math.MaxUint64/units {

		return PeriodReward{}, fmt.Errorf(
			"participation reward amount overflow",
		)
	}

	amount :=
		units * policy.ParticipationReward

	return PeriodReward{
		Address: address,
		Period:  periodIndex,
		Points:  points,
		Units:   units,
		Amount:  amount,
	}, nil
}
