package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

type grantReplayKey struct {
	Pool consensus.ReservedPool
}

func (state *ReplayState) ValidateGrantNext(
	grant Grant,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if grant.ID == "" {
		return fmt.Errorf(
			"reserved grant ID cannot be empty",
		)
	}

	if grant.Nonce == 0 {
		return fmt.Errorf(
			"reserved grant nonce must be greater than zero",
		)
	}

	revoked, err :=
		state.IsGrantRevoked(grant)

	if err != nil {
		return err
	}

	if revoked {
		return fmt.Errorf(
			"reserved grant has been revoked",
		)
	}

	if state.usedGrantIDs != nil {
		if _, exists := state.usedGrantIDs[grant.ID]; exists {
			return fmt.Errorf(
				"reserved grant already used",
			)
		}
	}

	key := grantReplayKey{
		Pool: grant.Pool,
	}

	if state.lastGrantNonce != nil {
		if last, exists := state.lastGrantNonce[key]; exists &&
			grant.Nonce <= last {

			return fmt.Errorf(
				"reserved grant nonce is not increasing: nonce=%d last=%d",
				grant.Nonce,
				last,
			)
		}
	}

	return nil
}

func (state *ReplayState) AcceptGrant(
	grant Grant,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if err := state.ValidateGrantNext(grant); err != nil {
		return err
	}

	if state.usedGrantIDs == nil {
		state.usedGrantIDs =
			make(map[string]struct{})
	}

	if state.lastGrantNonce == nil {
		state.lastGrantNonce =
			make(map[grantReplayKey]uint64)
	}

	key := grantReplayKey{
		Pool: grant.Pool,
	}

	state.usedGrantIDs[grant.ID] =
		struct{}{}

	state.lastGrantNonce[key] =
		grant.Nonce

	return nil
}
