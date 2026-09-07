package participation

const (
	RewardPointsPerUnit     uint64 = 10
	MaxRewardUnitsPerPeriod uint64 = 10
)

// RewardUnits converts newly earned PoUP points into
// bounded reward units.
//
// The input must represent new participation points for
// the current reward period, not an address's lifetime score.
func RewardUnits(
	newPoints uint64,
) uint64 {
	units := newPoints / RewardPointsPerUnit

	if units > MaxRewardUnitsPerPeriod {
		return MaxRewardUnitsPerPeriod
	}

	return units
}
