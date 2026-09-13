package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

type revocationReplayKey struct {
	Pool    consensus.ReservedPool
	GrantID string
}

func (state *ReplayState) ValidateRevocationNext(
	revocation Revocation,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if err :=
		ValidateRevocation(revocation); err != nil {

		return err
	}

	if state.usedGrantIDs != nil {
		if _, exists :=
			state.usedGrantIDs[revocation.GrantID]; exists {

			return fmt.Errorf(
				"cannot revoke executed reserved grant",
			)
		}
	}

	key := revocationReplayKey{
		Pool:    revocation.Pool,
		GrantID: revocation.GrantID,
	}

	if state.revokedGrants != nil {
		if _, exists :=
			state.revokedGrants[key]; exists {

			return fmt.Errorf(
				"reserved grant already revoked",
			)
		}
	}

	return nil
}

func (state *ReplayState) AcceptRevocation(
	revocation Revocation,
	policy AuthorityPolicy,
	expectedChainID string,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if err :=
		policy.ValidateRevocation(
			revocation,
			expectedChainID,
		); err != nil {

		return err
	}

	if err :=
		state.ValidateRevocationNext(
			revocation,
		); err != nil {

		return err
	}

	if state.revokedGrants == nil {
		state.revokedGrants =
			make(
				map[revocationReplayKey]struct{},
			)
	}

	key := revocationReplayKey{
		Pool:    revocation.Pool,
		GrantID: revocation.GrantID,
	}

	state.revokedGrants[key] =
		struct{}{}

	return nil
}

func (state *ReplayState) IsGrantRevoked(
	grant Grant,
) (bool, error) {
	if state == nil {
		return false, fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if grant.ID == "" {
		return false, fmt.Errorf(
			"reserved grant ID cannot be empty",
		)
	}

	if state.revokedGrants == nil {
		return false, nil
	}

	key := revocationReplayKey{
		Pool:    grant.Pool,
		GrantID: grant.ID,
	}

	_, exists :=
		state.revokedGrants[key]

	return exists, nil
}
