package reserved

import (
	"fmt"
	"sort"

	"prism/internal/consensus"
)

func cloneAuthorityPolicy(
	policy AuthorityPolicy,
) AuthorityPolicy {
	return AuthorityPolicy{
		Ecosystem: append(
			[]string(nil),
			policy.Ecosystem...,
		),
		Treasury: append(
			[]string(nil),
			policy.Treasury...,
		),
		Team: append(
			[]string(nil),
			policy.Team...,
		),
		Liquidity: append(
			[]string(nil),
			policy.Liquidity...,
		),

		EcosystemThreshold: policy.EcosystemThreshold,
		TreasuryThreshold:  policy.TreasuryThreshold,
		TeamThreshold:      policy.TeamThreshold,
		LiquidityThreshold: policy.LiquidityThreshold,
	}
}

func setAuthorityList(
	policy *AuthorityPolicy,
	pool consensus.ReservedPool,
	authorities []string,
) error {
	if policy == nil {
		return fmt.Errorf(
			"reserved authority policy cannot be nil",
		)
	}

	switch pool {
	case consensus.ReservedPoolEcosystem:
		policy.Ecosystem = authorities

	case consensus.ReservedPoolTreasury:
		policy.Treasury = authorities

	case consensus.ReservedPoolTeam:
		policy.Team = authorities

	case consensus.ReservedPoolLiquidity:
		policy.Liquidity = authorities

	default:
		return fmt.Errorf(
			"unknown reserved pool: %q",
			pool,
		)
	}

	return nil
}

// ApplyAuthorityChange validates a governance change against the
// currently active authority set and returns a new policy.
//
// The receiver is never mutated.
func (policy AuthorityPolicy) ApplyAuthorityChange(
	change AuthorityChange,
	expectedChainID string,
) (
	AuthorityPolicy,
	error,
) {
	if err :=
		policy.ValidateAuthorityChange(
			change,
			expectedChainID,
		); err != nil {

		return AuthorityPolicy{}, err
	}

	current, err :=
		policy.authorities(
			change.Pool,
		)

	if err != nil {
		return AuthorityPolicy{}, err
	}

	foundIndex := -1

	for index, authority := range current {

		if authority == change.Authority {
			foundIndex = index
			break
		}
	}

	next :=
		append(
			[]string(nil),
			current...,
		)

	switch change.Action {
	case AuthorityChangeAdd:
		if foundIndex >= 0 {
			return AuthorityPolicy{},
				fmt.Errorf(
					"reserved authority is already configured for pool %q",
					change.Pool,
				)
		}

		next =
			append(
				next,
				change.Authority,
			)

	case AuthorityChangeRemove:
		if foundIndex < 0 {
			return AuthorityPolicy{},
				fmt.Errorf(
					"reserved authority is not configured for pool %q",
					change.Pool,
				)
		}

		if len(next) == 1 {
			return AuthorityPolicy{},
				fmt.Errorf(
					"cannot remove last reserved authority from pool %q",
					change.Pool,
				)
		}

		next =
			append(
				next[:foundIndex],
				next[foundIndex+1:]...,
			)

	default:
		return AuthorityPolicy{},
			fmt.Errorf(
				"invalid reserved authority change action: %q",
				change.Action,
			)
	}

	// Keep the authority set deterministic, matching ChainConfig
	// canonical ordering.
	sort.Strings(next)

	updated :=
		cloneAuthorityPolicy(
			policy,
		)

	if err :=
		setAuthorityList(
			&updated,
			change.Pool,
			next,
		); err != nil {

		return AuthorityPolicy{}, err
	}

	if err := updated.Validate(); err != nil {
		return AuthorityPolicy{},
			fmt.Errorf(
				"authority change would invalidate reserved policy: %w",
				err,
			)
	}

	return updated, nil
}
