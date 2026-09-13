package participation

import "prism/internal/poup"

const RewardPeriodBlocks uint64 = poup.RewardPeriodBlocks

type RewardPeriod = poup.RewardPeriod

func PeriodForHeight(
	height uint64,
) (RewardPeriod, error) {
	return poup.PeriodForHeight(
		height,
	)
}

func PeriodByIndex(
	index uint64,
) (RewardPeriod, error) {
	return poup.PeriodByIndex(
		index,
	)
}
