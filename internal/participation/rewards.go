package participation

import "prism/internal/poup"

const (
	RewardPointsPerUnit uint64 = poup.RewardPointsPerUnit

	MaxRewardUnitsPerPeriod uint64 = poup.MaxRewardUnitsPerPeriod
)

// RewardUnits converts newly earned PoUP points into
// bounded reward units.
//
// The input must represent new participation points for
// the current reward period, not an address's lifetime score.
func RewardUnits(
	newPoints uint64,
) uint64 {
	return poup.RewardUnits(
		newPoints,
	)
}
