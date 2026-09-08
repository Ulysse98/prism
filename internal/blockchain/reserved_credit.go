package blockchain

import (
	"fmt"
	"math"

	"prism/internal/reserved"
)

func creditReservedAuthorization(
	state *State,
	authorization reserved.Authorization,
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
		state.Balances[authorization.Recipient]

	if current >
		math.MaxUint64-authorization.Amount {

		return fmt.Errorf(
			"reserved authorization balance overflow for %s",
			authorization.Recipient,
		)
	}

	state.Balances[authorization.Recipient] =
		current + authorization.Amount

	return nil
}

func creditReservedGrant(
	state *State,
	grant reserved.Grant,
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
		state.Balances[grant.Recipient]

	if current >
		math.MaxUint64-grant.Amount {

		return fmt.Errorf(
			"reserved grant balance overflow for %s",
			grant.Recipient,
		)
	}

	state.Balances[grant.Recipient] =
		current + grant.Amount

	return nil
}
