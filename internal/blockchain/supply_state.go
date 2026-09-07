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
	ReservedAllocation      uint64 `json:"reservedAllocation"`
	RemainingSupply         uint64 `json:"remainingSupply"`
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

	if networkEmission > ledgerSupply {
		return SupplyState{}, fmt.Errorf(
			"network emission exceeds ledger supply",
		)
	}

	if ledgerSupply > policy.MaxSupply {
		return SupplyState{}, fmt.Errorf(
			"ledger supply exceeds maximum supply: supply=%d max=%d",
			ledgerSupply,
			policy.MaxSupply,
		)
	}

	return SupplyState{
		MaxSupply:               policy.MaxSupply,
		LedgerSupply:            ledgerSupply,
		GenesisSupply:           genesisSupply,
		NetworkEmission:         networkEmission,
		NetworkRewardAllocation: policy.NetworkRewardAllocation(),
		ReservedAllocation:      policy.ReservedAllocation(),
		RemainingSupply:         policy.MaxSupply - ledgerSupply,
	}, nil
}
