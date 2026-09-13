package consensus

import "fmt"

type ReservedPool string

const (
	ReservedPoolEcosystem ReservedPool = "ecosystem"
	ReservedPoolTreasury  ReservedPool = "treasury"
	ReservedPoolTeam      ReservedPool = "team"
	ReservedPoolLiquidity ReservedPool = "liquidity"
)

func (policy SupplyPolicy) ReservedPoolAllocation(
	pool ReservedPool,
) (uint64, error) {
	if err := policy.Validate(); err != nil {
		return 0, err
	}

	switch pool {
	case ReservedPoolEcosystem:
		return policy.EcosystemAllocation, nil

	case ReservedPoolTreasury:
		return policy.TreasuryAllocation, nil

	case ReservedPoolTeam:
		return policy.TeamAllocation, nil

	case ReservedPoolLiquidity:
		return policy.LiquidityAllocation, nil

	default:
		return 0, fmt.Errorf(
			"unknown reserved pool: %q",
			pool,
		)
	}
}
