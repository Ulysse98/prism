package blockchain

import (
	"fmt"
	"math"

	"prism/internal/consensus"
	"prism/internal/transaction"
)

type SupplyState struct {
	MaxSupply               uint64 `json:"maxSupply"`
	LedgerSupply            uint64 `json:"ledgerSupply"`
	GenesisSupply           uint64 `json:"genesisSupply"`
	NetworkEmission         uint64 `json:"networkEmission"`
	NetworkRewardAllocation uint64 `json:"networkRewardAllocation"`
	NetworkRewardRemaining  uint64 `json:"networkRewardRemaining"`

	ReservedAllocation        uint64 `json:"reservedAllocation"`
	ReservedConsumedByGenesis uint64 `json:"reservedConsumedByGenesis"`
	ReservedRemaining         uint64 `json:"reservedRemaining"`

	EcosystemAllocation uint64 `json:"ecosystemAllocation"`
	TreasuryAllocation  uint64 `json:"treasuryAllocation"`
	TeamAllocation      uint64 `json:"teamAllocation"`
	LiquidityAllocation uint64 `json:"liquidityAllocation"`

	EcosystemRemaining uint64 `json:"ecosystemRemaining"`
	TreasuryRemaining  uint64 `json:"treasuryRemaining"`
	TeamRemaining      uint64 `json:"teamRemaining"`
	LiquidityRemaining uint64 `json:"liquidityRemaining"`

	RemainingSupply uint64 `json:"remainingSupply"`
}

func (bc *Blockchain) GenesisSupply() (
	uint64,
	error,
) {
	if bc == nil {
		return 0, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if len(bc.Blocks) == 0 {
		return 0, fmt.Errorf(
			"blockchain has no genesis block",
		)
	}

	genesis := bc.Blocks[0]

	if genesis.Height != 0 ||
		genesis.Proposer != "GENESIS" {

		return 0, fmt.Errorf(
			"invalid genesis block",
		)
	}

	var total uint64

	for _, tx := range genesis.Transactions {
		if err := transaction.ValidateGenesis(
			tx,
		); err != nil {
			return 0, fmt.Errorf(
				"invalid genesis transaction: %w",
				err,
			)
		}

		if total > math.MaxUint64-tx.Amount {
			return 0, fmt.Errorf(
				"genesis supply overflow",
			)
		}

		total += tx.Amount
	}

	return total, nil
}

func (bc *Blockchain) GetSupplyState() (
	SupplyState,
	error,
) {
	if bc == nil {
		return SupplyState{}, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	policy :=
		consensus.DefaultSupplyPolicy()

	if err := policy.Validate(); err != nil {
		return SupplyState{}, fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	ledgerSupply, err :=
		bc.TotalSupply()

	if err != nil {
		return SupplyState{}, fmt.Errorf(
			"cannot calculate ledger supply: %w",
			err,
		)
	}

	genesisSupply, err :=
		bc.GenesisSupply()

	if err != nil {
		return SupplyState{}, fmt.Errorf(
			"cannot calculate genesis supply: %w",
			err,
		)
	}

	emission, err :=
		bc.GetEmissionState()

	if err != nil {
		return SupplyState{}, fmt.Errorf(
			"cannot calculate network emission: %w",
			err,
		)
	}

	networkEmission, err :=
		emission.NetworkEmission()

	if err != nil {
		return SupplyState{}, err
	}

	rewardPools, err :=
		emission.RemainingRewardPools(
			policy,
		)

	if err != nil {
		return SupplyState{}, fmt.Errorf(
			"cannot calculate remaining reward pools: %w",
			err,
		)
	}

	networkRewardRemaining, err :=
		rewardPools.NetworkRemaining()

	if err != nil {
		return SupplyState{}, err
	}

	reservedUsage :=
		consensus.ReservedUsage{
			LegacyGenesis: genesisSupply,
		}

	reservedBudget, err :=
		reservedUsage.Remaining(
			policy,
		)

	if err != nil {
		return SupplyState{}, fmt.Errorf(
			"cannot calculate reserved budget: %w",
			err,
		)
	}

	reservedRemaining :=
		reservedBudget.TotalRemaining

	if genesisSupply >
		math.MaxUint64-networkEmission {

		return SupplyState{}, fmt.Errorf(
			"supply accounting overflow",
		)
	}

	accountedLedger :=
		genesisSupply +
			networkEmission

	if ledgerSupply != accountedLedger {
		return SupplyState{}, fmt.Errorf(
			"ledger supply is not fully accounted: ledger=%d accounted=%d",
			ledgerSupply,
			accountedLedger,
		)
	}

	if ledgerSupply > policy.MaxSupply {
		return SupplyState{}, fmt.Errorf(
			"ledger supply exceeds maximum supply: supply=%d max=%d",
			ledgerSupply,
			policy.MaxSupply,
		)
	}

	if networkRewardRemaining >
		math.MaxUint64-reservedRemaining {

		return SupplyState{}, fmt.Errorf(
			"remaining supply accounting overflow",
		)
	}

	futureMintable :=
		networkRewardRemaining +
			reservedRemaining

	remainingSupply :=
		policy.MaxSupply -
			ledgerSupply

	if futureMintable != remainingSupply {
		return SupplyState{}, fmt.Errorf(
			"remaining supply accounting mismatch: remaining=%d mintable=%d",
			remainingSupply,
			futureMintable,
		)
	}

	return SupplyState{
		MaxSupply:               policy.MaxSupply,
		LedgerSupply:            ledgerSupply,
		GenesisSupply:           genesisSupply,
		NetworkEmission:         networkEmission,
		NetworkRewardAllocation: policy.NetworkRewardAllocation(),
		NetworkRewardRemaining:  networkRewardRemaining,

		ReservedAllocation:        policy.ReservedAllocation(),
		ReservedConsumedByGenesis: genesisSupply,
		ReservedRemaining:         reservedRemaining,

		EcosystemAllocation: policy.EcosystemAllocation,
		TreasuryAllocation:  policy.TreasuryAllocation,
		TeamAllocation:      policy.TeamAllocation,
		LiquidityAllocation: policy.LiquidityAllocation,

		EcosystemRemaining: reservedBudget.EcosystemRemaining,
		TreasuryRemaining:  reservedBudget.TreasuryRemaining,
		TeamRemaining:      reservedBudget.TeamRemaining,
		LiquidityRemaining: reservedBudget.LiquidityRemaining,

		RemainingSupply: remainingSupply,
	}, nil
}
