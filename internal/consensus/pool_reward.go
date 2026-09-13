package consensus

import "fmt"

// BoundedPoolReward returns the amount that can still be
// emitted without exceeding a finite reward pool.
func BoundedPoolReward(
	baseReward uint64,
	emitted uint64,
	pool uint64,
) (uint64, error) {
	if emitted > pool {
		return 0, fmt.Errorf(
			"emission exceeds reward pool",
		)
	}

	remaining := pool - emitted

	if baseReward > remaining {
		return remaining, nil
	}

	return baseReward, nil
}
