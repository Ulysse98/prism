package consensus

import "fmt"

const (
	MaxSupply uint64 = 100_000_000

	ProposerRewardPool      uint64 = 20_000_000
	UsefulWorkRewardPool    uint64 = 20_000_000
	ParticipationRewardPool uint64 = 20_000_000

	EcosystemAllocation uint64 = 15_000_000
	TreasuryAllocation  uint64 = 10_000_000
	TeamAllocation      uint64 = 10_000_000
	LiquidityAllocation uint64 = 5_000_000
)

type SupplyPolicy struct {
	MaxSupply uint64

	ProposerRewardPool      uint64
	UsefulWorkRewardPool    uint64
	ParticipationRewardPool uint64

	EcosystemAllocation uint64
	TreasuryAllocation  uint64
	TeamAllocation      uint64
	LiquidityAllocation uint64
}

func DefaultSupplyPolicy() SupplyPolicy {
	return SupplyPolicy{
		MaxSupply: MaxSupply,

		ProposerRewardPool:      ProposerRewardPool,
		UsefulWorkRewardPool:    UsefulWorkRewardPool,
		ParticipationRewardPool: ParticipationRewardPool,

		EcosystemAllocation: EcosystemAllocation,
		TreasuryAllocation:  TreasuryAllocation,
		TeamAllocation:      TeamAllocation,
		LiquidityAllocation: LiquidityAllocation,
	}
}

func (policy SupplyPolicy) NetworkRewardAllocation() uint64 {
	return policy.ProposerRewardPool +
		policy.UsefulWorkRewardPool +
		policy.ParticipationRewardPool
}

func (policy SupplyPolicy) ReservedAllocation() uint64 {
	return policy.EcosystemAllocation +
		policy.TreasuryAllocation +
		policy.TeamAllocation +
		policy.LiquidityAllocation
}

func (policy SupplyPolicy) Validate() error {
	if policy.MaxSupply == 0 {
		return fmt.Errorf(
			"maximum supply must be greater than zero",
		)
	}

	if policy.ProposerRewardPool == 0 ||
		policy.UsefulWorkRewardPool == 0 ||
		policy.ParticipationRewardPool == 0 {

		return fmt.Errorf(
			"network reward pools must be greater than zero",
		)
	}

	total :=
		policy.NetworkRewardAllocation() +
			policy.ReservedAllocation()

	if total != policy.MaxSupply {
		return fmt.Errorf(
			"supply allocation mismatch: allocated=%d max=%d",
			total,
			policy.MaxSupply,
		)
	}

	return nil
}
