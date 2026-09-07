package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

type AuthorityPolicy struct {
	Ecosystem []string
	Treasury  []string
	Team      []string
	Liquidity []string
}

func DefaultAuthorityPolicy() AuthorityPolicy {
	return AuthorityPolicy{}
}

func (policy AuthorityPolicy) authorities(
	pool consensus.ReservedPool,
) ([]string, error) {
	switch pool {
	case consensus.ReservedPoolEcosystem:
		return policy.Ecosystem, nil

	case consensus.ReservedPoolTreasury:
		return policy.Treasury, nil

	case consensus.ReservedPoolTeam:
		return policy.Team, nil

	case consensus.ReservedPoolLiquidity:
		return policy.Liquidity, nil

	default:
		return nil, fmt.Errorf(
			"unknown reserved pool: %q",
			pool,
		)
	}
}

func (policy AuthorityPolicy) IsAuthorized(
	pool consensus.ReservedPool,
	address string,
) (bool, error) {
	if address == "" {
		return false, fmt.Errorf(
			"reserved authority address cannot be empty",
		)
	}

	authorities, err :=
		policy.authorities(pool)

	if err != nil {
		return false, err
	}

	for _, authority := range authorities {

		if authority == address {
			return true, nil
		}
	}

	return false, nil
}

func (policy AuthorityPolicy) ValidateAuthorization(
	authorization Authorization,
	expectedChainID string,
) error {
	if err :=
		ValidateSignedForChain(
			authorization,
			expectedChainID,
		); err != nil {

		return err
	}

	authorized, err :=
		policy.IsAuthorized(
			authorization.Pool,
			authorization.Authorizer,
		)

	if err != nil {
		return err
	}

	if !authorized {
		return fmt.Errorf(
			"authorizer is not authorized for reserved pool",
		)
	}

	return nil
}
