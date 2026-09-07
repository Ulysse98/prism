package consensus

import "fmt"

type ReservedUsage struct {
	LegacyGenesis uint64

	Ecosystem uint64
	Treasury  uint64
	Team      uint64
	Liquidity uint64
}

type ReservedBudgetState struct {
	LegacyGenesis uint64

	EcosystemRemaining uint64
	TreasuryRemaining  uint64
	TeamRemaining      uint64
	LiquidityRemaining uint64

	TotalRemaining uint64
}

func (usage ReservedUsage) ExplicitTotal() (
	uint64,
	error,
) {
	return checkedSupplySum(
		usage.Ecosystem,
		usage.Treasury,
		usage.Team,
		usage.Liquidity,
	)
}

func (usage ReservedUsage) Validate(
	policy SupplyPolicy,
) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	if usage.Ecosystem >
		policy.EcosystemAllocation {

		return fmt.Errorf(
			"ecosystem usage exceeds allocation",
		)
	}

	if usage.Treasury >
		policy.TreasuryAllocation {

		return fmt.Errorf(
			"treasury usage exceeds allocation",
		)
	}

	if usage.Team >
		policy.TeamAllocation {

		return fmt.Errorf(
			"team usage exceeds allocation",
		)
	}

	if usage.Liquidity >
		policy.LiquidityAllocation {

		return fmt.Errorf(
			"liquidity usage exceeds allocation",
		)
	}

	explicit, err :=
		usage.ExplicitTotal()

	if err != nil {
		return fmt.Errorf(
			"reserved usage overflow: %w",
			err,
		)
	}

	total, err :=
		checkedSupplySum(
			usage.LegacyGenesis,
			explicit,
		)

	if err != nil {
		return fmt.Errorf(
			"reserved usage overflow: %w",
			err,
		)
	}

	if total > policy.ReservedAllocation() {
		return fmt.Errorf(
			"reserved usage exceeds allocation: used=%d allocation=%d",
			total,
			policy.ReservedAllocation(),
		)
	}

	return nil
}

func (usage ReservedUsage) Remaining(
	policy SupplyPolicy,
) (
	ReservedBudgetState,
	error,
) {
	if err := usage.Validate(
		policy,
	); err != nil {
		return ReservedBudgetState{}, err
	}

	explicit, err :=
		usage.ExplicitTotal()

	if err != nil {
		return ReservedBudgetState{}, err
	}

	totalUsed, err :=
		checkedSupplySum(
			usage.LegacyGenesis,
			explicit,
		)

	if err != nil {
		return ReservedBudgetState{}, err
	}

	return ReservedBudgetState{
		LegacyGenesis: usage.LegacyGenesis,

		EcosystemRemaining: policy.EcosystemAllocation -
			usage.Ecosystem,

		TreasuryRemaining: policy.TreasuryAllocation -
			usage.Treasury,

		TeamRemaining: policy.TeamAllocation -
			usage.Team,

		LiquidityRemaining: policy.LiquidityAllocation -
			usage.Liquidity,

		TotalRemaining: policy.ReservedAllocation() -
			totalUsed,
	}, nil
}
