package consensus

const UsefulWorkScoreRewardActivationHeight uint64 = 29

// UsefulWorkBaseReward returns the consensus PoUW reward before
// applying the finite useful-work reward-pool cap.
//
// Blocks before the activation height retain the legacy fixed reward
// so existing chain history remains deterministic.
func UsefulWorkBaseReward(
	height uint64,
	workUnits uint64,
	policy RewardPolicy,
) uint64 {
	if height < UsefulWorkScoreRewardActivationHeight {
		return policy.UsefulWorkReward
	}

	switch {
	case workUnits == 0:
		return 0

	case workUnits <= 4:
		return 1

	case workUnits <= 8:
		return 2

	case workUnits <= 16:
		return 3

	default:
		return 4
	}
}
