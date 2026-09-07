package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestSupplyStateAtGenesis(
	t *testing.T,
) {
	chain, err := NewBlockchain(
		map[string]uint64{
			"Alice": 1000,
			"Bob":   250,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	state, err :=
		chain.GetSupplyState()

	if err != nil {
		t.Fatal(err)
	}

	if state.MaxSupply != 100_000_000 {
		t.Fatalf(
			"expected max supply 100000000, got %d",
			state.MaxSupply,
		)
	}

	if state.LedgerSupply != 1250 {
		t.Fatalf(
			"expected ledger supply 1250, got %d",
			state.LedgerSupply,
		)
	}

	if state.GenesisSupply != 1250 {
		t.Fatalf(
			"expected genesis supply 1250, got %d",
			state.GenesisSupply,
		)
	}

	if state.NetworkEmission != 0 {
		t.Fatalf(
			"expected zero network emission, got %d",
			state.NetworkEmission,
		)
	}

	if state.NetworkRewardAllocation != 60_000_000 {
		t.Fatalf(
			"expected network allocation 60000000, got %d",
			state.NetworkRewardAllocation,
		)
	}

	if state.ReservedAllocation != 40_000_000 {
		t.Fatalf(
			"expected reserved allocation 40000000, got %d",
			state.ReservedAllocation,
		)
	}

	if state.RemainingSupply != 99_998_750 {
		t.Fatalf(
			"expected remaining supply 99998750, got %d",
			state.RemainingSupply,
		)
	}
}

func TestSupplyStateTracksNetworkEmission(
	t *testing.T,
) {
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := NewBlockchain(
		map[string]uint64{
			alice.Address: 100,
			bob.Address:   0,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := chain.LockStake(
		alice.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos :=
		consensus.NewProofOfStake()

	if err := pos.Register(
		alice.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	tx :=
		transaction.New(
			alice.Address,
			bob.Address,
			1,
			0,
			alice.PublicKeyHex(),
		)

	if err := tx.Sign(
		alice.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := chain.AddBlock(
		[]transaction.Transaction{
			tx,
		},
		nil,
		alice.Address,
		pos,
	); err != nil {
		t.Fatal(err)
	}

	state, err :=
		chain.GetSupplyState()

	if err != nil {
		t.Fatal(err)
	}

	if state.GenesisSupply != 100 {
		t.Fatalf(
			"expected genesis supply 100, got %d",
			state.GenesisSupply,
		)
	}

	if state.NetworkEmission != 5 {
		t.Fatalf(
			"expected network emission 5, got %d",
			state.NetworkEmission,
		)
	}

	if state.LedgerSupply != 105 {
		t.Fatalf(
			"expected ledger supply 105, got %d",
			state.LedgerSupply,
		)
	}

	if state.RemainingSupply != 99_999_895 {
		t.Fatalf(
			"expected remaining supply 99999895, got %d",
			state.RemainingSupply,
		)
	}
}

func TestSupplyStateRejectsSupplyAboveMaximum(
	t *testing.T,
) {
	chain, err := NewBlockchain(
		map[string]uint64{
			"Alice": 100_000_001,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	if _, err :=
		chain.GetSupplyState(); err == nil {

		t.Fatal(
			"expected supply above maximum to fail",
		)
	}
}

func TestGenesisSupplyReadsGenesisBlock(
	t *testing.T,
) {
	chain, err := NewBlockchain(
		map[string]uint64{
			"Alice": 123,
			"Bob":   456,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	supply, err :=
		chain.GenesisSupply()

	if err != nil {
		t.Fatal(err)
	}

	if supply != 579 {
		t.Fatalf(
			"expected genesis supply 579, got %d",
			supply,
		)
	}
}

func TestGenesisSupplyRejectsInvalidGenesisTransaction(
	t *testing.T,
) {
	chain, err := NewBlockchain(
		map[string]uint64{
			"Alice": 100,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	chain.Blocks[0].Transactions[0].Amount++

	if _, err :=
		chain.GenesisSupply(); err == nil {

		t.Fatal(
			"expected invalid genesis transaction to fail",
		)
	}
}
