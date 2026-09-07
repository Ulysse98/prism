package participation

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

func CalculatePeriod(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	eligibility EligibilityChecker,
	periodIndex uint64,
) ([]Score, error) {
	period, err := PeriodByIndex(
		periodIndex,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid reward period: %w",
			err,
		)
	}

	return calculateRange(
		chain,
		pos,
		eligibility,
		period.StartHeight,
		period.EndHeight,
	)
}

func RewardUnitsForAddress(
	scores []Score,
	address string,
) uint64 {
	return RewardUnits(
		ScoreOf(scores, address),
	)
}
