package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func signedReservedReplayAuthorization(
	t *testing.T,
	bc *Blockchain,
	authorizer *wallet.Wallet,
	nonce uint64,
	amount uint64,
) reserved.Authorization {
	t.Helper()

	chainID, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	recipient, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		reserved.NewAuthorization(
			chainID,
			nonce,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			amount,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	return authorization
}

func TestReservedAccountingReplayStartsFromGenesis(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1250,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	state, err :=
		bc.GetReservedAccountingState(
			reserved.DefaultAuthorityPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if state.Usage.LegacyGenesis != 1250 {
		t.Fatalf(
			"expected legacy genesis usage 1250, got %d",
			state.Usage.LegacyGenesis,
		)
	}

	budget, err :=
		state.Budget(
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if budget.TotalRemaining != 39_998_750 {
		t.Fatalf(
			"expected reserved remaining 39998750, got %d",
			budget.TotalRemaining,
		)
	}
}

func TestReservedAccountingReplayTracksAuthorizations(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1250,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			1,
			100,
		)

	second :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			2,
			250,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			ReservedAuthorizations: []reserved.Authorization{
				first,
			},
		},
		Block{
			Height: 2,
			ReservedAuthorizations: []reserved.Authorization{
				second,
			},
		},
	)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	state, err :=
		bc.GetReservedAccountingState(
			policy,
		)

	if err != nil {
		t.Fatal(err)
	}

	if state.Usage.Treasury != 350 {
		t.Fatalf(
			"expected treasury usage 350, got %d",
			state.Usage.Treasury,
		)
	}

	budget, err :=
		state.Budget(
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if budget.TreasuryRemaining != 9_999_650 {
		t.Fatalf(
			"expected treasury remaining 9999650, got %d",
			budget.TreasuryRemaining,
		)
	}

	if budget.TotalRemaining != 39_998_400 {
		t.Fatalf(
			"expected total remaining 39998400, got %d",
			budget.TotalRemaining,
		)
	}
}

func TestReservedAccountingReplayRejectsReusedNonce(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			1,
			100,
		)

	second :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			1,
			200,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			ReservedAuthorizations: []reserved.Authorization{
				first,
			},
		},
		Block{
			Height: 2,
			ReservedAuthorizations: []reserved.Authorization{
				second,
			},
		},
	)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	if _, err :=
		bc.GetReservedAccountingState(
			policy,
		); err == nil {

		t.Fatal(
			"expected reused reserved nonce to fail replay",
		)
	}
}

func TestReservedAccountingReplayRejectsCumulativePoolOverflow(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			1,
			6_000_000,
		)

	second :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			2,
			5_000_000,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			ReservedAuthorizations: []reserved.Authorization{
				first,
			},
		},
		Block{
			Height: 2,
			ReservedAuthorizations: []reserved.Authorization{
				second,
			},
		},
	)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	if _, err :=
		bc.GetReservedAccountingState(
			policy,
		); err == nil {

		t.Fatal(
			"expected cumulative treasury overflow to fail replay",
		)
	}
}
