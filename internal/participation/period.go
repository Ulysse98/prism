package participation

import (
	"fmt"
	"math"
)

const RewardPeriodBlocks uint64 = 100

type RewardPeriod struct {
	Index       uint64
	StartHeight uint64
	EndHeight   uint64
}

func PeriodForHeight(
	height uint64,
) (RewardPeriod, error) {
	if height == 0 {
		return RewardPeriod{}, fmt.Errorf(
			"genesis block does not belong to a reward period",
		)
	}

	index :=
		(height - 1) / RewardPeriodBlocks

	return PeriodByIndex(index)
}

func PeriodByIndex(
	index uint64,
) (RewardPeriod, error) {
	if index >
		(math.MaxUint64-1)/RewardPeriodBlocks {

		return RewardPeriod{}, fmt.Errorf(
			"reward period height overflow",
		)
	}

	start :=
		index*RewardPeriodBlocks + 1

	if start >
		math.MaxUint64-(RewardPeriodBlocks-1) {

		return RewardPeriod{}, fmt.Errorf(
			"reward period end height overflow",
		)
	}

	end :=
		start + RewardPeriodBlocks - 1

	return RewardPeriod{
		Index:       index,
		StartHeight: start,
		EndHeight:   end,
	}, nil
}
