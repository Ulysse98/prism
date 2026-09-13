package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestAccountingStateTracksReservedUsage(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTreasury,
			1,
			100,
		)

	authorityPolicy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	state :=
		NewAccountingState(1250)

	if err :=
		state.Accept(
			authorization,
			authorityPolicy,
			testChainID,
			supplyPolicy,
		); err != nil {

		t.Fatal(err)
	}

	if state.Usage.Treasury != 100 {
		t.Fatalf(
			"expected treasury usage 100, got %d",
			state.Usage.Treasury,
		)
	}

	budget, err :=
		state.Budget(
			supplyPolicy,
		)

	if err != nil {
		t.Fatal(err)
	}

	if budget.TreasuryRemaining != 9_999_900 {
		t.Fatalf(
			"expected treasury remaining 9999900, got %d",
			budget.TreasuryRemaining,
		)
	}

	if budget.TotalRemaining != 39_998_650 {
		t.Fatalf(
			"expected total remaining 39998650, got %d",
			budget.TotalRemaining,
		)
	}
}

func TestAccountingStatePoolFailureDoesNotConsumeNonce(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolLiquidity,
			1,
			4_999_900,
		)

	tooMuch :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolLiquidity,
			2,
			200,
		)

	exactFinal :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolLiquidity,
			2,
			100,
		)

	authorityPolicy := AuthorityPolicy{
		Liquidity: []string{
			authorizer.Address,
		},
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	state :=
		NewAccountingState(0)

	if err :=
		state.Accept(
			first,
			authorityPolicy,
			testChainID,
			supplyPolicy,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.Accept(
			tooMuch,
			authorityPolicy,
			testChainID,
			supplyPolicy,
		); err == nil {

		t.Fatal(
			"expected cumulative liquidity overflow to fail",
		)
	}

	if err :=
		state.Accept(
			exactFinal,
			authorityPolicy,
			testChainID,
			supplyPolicy,
		); err != nil {

		t.Fatal(
			"budget failure must not consume nonce",
		)
	}
}

func TestAccountingStateGlobalFailureDoesNotConsumeNonce(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	tooMuch :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTreasury,
			1,
			101,
		)

	exactFinal :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTreasury,
			1,
			100,
		)

	authorityPolicy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	state :=
		NewAccountingState(
			39_999_900,
		)

	if err :=
		state.Accept(
			tooMuch,
			authorityPolicy,
			testChainID,
			supplyPolicy,
		); err == nil {

		t.Fatal(
			"expected global reserved budget overflow to fail",
		)
	}

	if err :=
		state.Accept(
			exactFinal,
			authorityPolicy,
			testChainID,
			supplyPolicy,
		); err != nil {

		t.Fatal(
			"global budget failure must not consume nonce",
		)
	}

	budget, err :=
		state.Budget(
			supplyPolicy,
		)

	if err != nil {
		t.Fatal(err)
	}

	if budget.TotalRemaining != 0 {
		t.Fatalf(
			"expected exhausted reserved budget, got %d",
			budget.TotalRemaining,
		)
	}
}
