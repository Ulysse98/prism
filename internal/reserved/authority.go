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

	EcosystemThreshold uint32 `json:"EcosystemThreshold,omitempty"`
	TreasuryThreshold  uint32 `json:"TreasuryThreshold,omitempty"`
	TeamThreshold      uint32 `json:"TeamThreshold,omitempty"`
	LiquidityThreshold uint32 `json:"LiquidityThreshold,omitempty"`
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

func (policy AuthorityPolicy) threshold(
	pool consensus.ReservedPool,
) (uint32, error) {
	switch pool {
	case consensus.ReservedPoolEcosystem:
		return policy.EcosystemThreshold, nil

	case consensus.ReservedPoolTreasury:
		return policy.TreasuryThreshold, nil

	case consensus.ReservedPoolTeam:
		return policy.TeamThreshold, nil

	case consensus.ReservedPoolLiquidity:
		return policy.LiquidityThreshold, nil

	default:
		return 0, fmt.Errorf(
			"unknown reserved pool: %q",
			pool,
		)
	}
}

func (policy AuthorityPolicy) EffectiveThreshold(
	pool consensus.ReservedPool,
) (uint32, error) {
	authorities, err :=
		policy.authorities(pool)

	if err != nil {
		return 0, err
	}

	if len(authorities) == 0 {
		return 0, fmt.Errorf(
			"no reserved authorities configured for pool %q",
			pool,
		)
	}

	configured, err :=
		policy.threshold(pool)

	if err != nil {
		return 0, err
	}

	// Backward-compatible v0.21 behavior.
	//
	// A zero threshold means any single configured authority
	// can authorize the pool.
	if configured == 0 {
		return 1, nil
	}

	if configured > uint32(len(authorities)) {
		return 0, fmt.Errorf(
			"reserved threshold exceeds authority count for pool %q: threshold=%d authorities=%d",
			pool,
			configured,
			len(authorities),
		)
	}

	return configured, nil
}

func (policy AuthorityPolicy) Validate() error {
	pools := []consensus.ReservedPool{
		consensus.ReservedPoolEcosystem,
		consensus.ReservedPoolTreasury,
		consensus.ReservedPoolTeam,
		consensus.ReservedPoolLiquidity,
	}

	for _, pool := range pools {
		authorities, err :=
			policy.authorities(pool)

		if err != nil {
			return err
		}

		configured, err :=
			policy.threshold(pool)

		if err != nil {
			return err
		}

		if len(authorities) == 0 {
			if configured != 0 {
				return fmt.Errorf(
					"reserved threshold configured without authorities for pool %q",
					pool,
				)
			}

			continue
		}

		if configured > uint32(len(authorities)) {
			return fmt.Errorf(
				"reserved threshold exceeds authority count for pool %q: threshold=%d authorities=%d",
				pool,
				configured,
				len(authorities),
			)
		}
	}

	return nil
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

func (policy AuthorityPolicy) ValidateGrant(
	grant Grant,
	expectedChainID string,
) error {
	if err := ValidateGrant(grant); err != nil {
		return err
	}

	if expectedChainID == "" {
		return fmt.Errorf(
			"expected chain ID cannot be empty",
		)
	}

	if grant.ChainID != expectedChainID {
		return fmt.Errorf(
			"reserved grant chain ID mismatch",
		)
	}

	if err := policy.Validate(); err != nil {
		return err
	}

	authorities, err :=
		policy.authorities(grant.Pool)

	if err != nil {
		return err
	}

	threshold, err :=
		policy.EffectiveThreshold(grant.Pool)

	if err != nil {
		return err
	}

	allowed := make(
		map[string]struct{},
		len(authorities),
	)

	for _, authority := range authorities {
		allowed[authority] = struct{}{}
	}

	approved := make(
		map[string]struct{},
	)

	for _, approval := range grant.Approvals {
		if err :=
			ValidateApproval(
				grant,
				approval,
			); err != nil {

			return err
		}

		if _, exists :=
			allowed[approval.Authorizer]; !exists {

			return fmt.Errorf(
				"grant approver is not authorized for reserved pool",
			)
		}

		if _, exists :=
			approved[approval.Authorizer]; exists {

			return fmt.Errorf(
				"duplicate reserved grant approver",
			)
		}

		approved[approval.Authorizer] =
			struct{}{}
	}

	if uint32(len(approved)) < threshold {
		return fmt.Errorf(
			"reserved grant approval threshold not met: approvals=%d threshold=%d",
			len(approved),
			threshold,
		)
	}

	return nil
}
