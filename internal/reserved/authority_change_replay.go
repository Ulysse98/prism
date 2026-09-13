package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

type authorityChangeReplayKey struct {
	Pool consensus.ReservedPool
}

func (state *ReplayState) ValidateAuthorityChangeNext(
	change AuthorityChange,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if err :=
		ValidateAuthorityChange(change); err != nil {

		return err
	}

	if state.usedAuthorityChangeIDs != nil {
		if _, exists :=
			state.usedAuthorityChangeIDs[change.ID]; exists {

			return fmt.Errorf(
				"reserved authority change already used",
			)
		}
	}

	key :=
		authorityChangeReplayKey{
			Pool: change.Pool,
		}

	if state.lastAuthorityChangeNonce != nil {
		if last, exists :=
			state.lastAuthorityChangeNonce[key]; exists &&
			change.Nonce <= last {

			return fmt.Errorf(
				"reserved authority change nonce is not increasing: nonce=%d last=%d",
				change.Nonce,
				last,
			)
		}
	}

	return nil
}

// AcceptAuthorityChange validates the change against the currently
// active authority policy, applies it to a new policy value, and only
// then commits its replay protection state.
//
// The input policy is not mutated.
func (state *ReplayState) AcceptAuthorityChange(
	change AuthorityChange,
	policy AuthorityPolicy,
	expectedChainID string,
) (
	AuthorityPolicy,
	error,
) {
	if state == nil {
		return AuthorityPolicy{},
			fmt.Errorf(
				"reserved replay state cannot be nil",
			)
	}

	updated, err :=
		policy.ApplyAuthorityChange(
			change,
			expectedChainID,
		)

	if err != nil {
		return AuthorityPolicy{}, err
	}

	if err :=
		state.ValidateAuthorityChangeNext(
			change,
		); err != nil {

		return AuthorityPolicy{}, err
	}

	if state.usedAuthorityChangeIDs == nil {
		state.usedAuthorityChangeIDs =
			make(
				map[string]struct{},
			)
	}

	if state.lastAuthorityChangeNonce == nil {
		state.lastAuthorityChangeNonce =
			make(
				map[authorityChangeReplayKey]uint64,
			)
	}

	key :=
		authorityChangeReplayKey{
			Pool: change.Pool,
		}

	state.usedAuthorityChangeIDs[change.ID] =
		struct{}{}

	state.lastAuthorityChangeNonce[key] =
		change.Nonce

	return updated, nil
}
