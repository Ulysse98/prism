package blockchain

import (
	"fmt"
	"math"

	"prism/internal/poup"
)

func creditParticipationReward(
	state *State,
	claim poup.Claim,
) error {
	if state == nil {
		return fmt.Errorf(
			"state cannot be nil",
		)
	}

	if state.Balances == nil {
		return fmt.Errorf(
			"state balances cannot be nil",
		)
	}

	current :=
		state.Balances[claim.Address]

	if current >
		math.MaxUint64-claim.Amount {

		return fmt.Errorf(
			"participation reward balance overflow for %s",
			claim.Address,
		)
	}

	state.Balances[claim.Address] =
		current + claim.Amount

	return nil
}
