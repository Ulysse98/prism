package consensus

import "testing"

func TestReservedBudgetAccountsLegacyGenesis(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	usage := ReservedUsage{
		LegacyGenesis: 1250,
	}

	state, err :=
		usage.Remaining(policy)

	if err != nil {
		t.Fatal(err)
	}

	if state.TotalRemaining != 39_998_750 {
		t.Fatalf(
			"expected total remaining 39998750, got %d",
			state.TotalRemaining,
		)
	}

	if state.EcosystemRemaining != 15_000_000 ||
		state.TreasuryRemaining != 10_000_000 ||
		state.TeamRemaining != 10_000_000 ||
		state.LiquidityRemaining != 5_000_000 {

		t.Fatal(
			"legacy genesis must not be assigned to a specific pool",
		)
	}
}

func TestReservedBudgetTracksExplicitUsage(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	usage := ReservedUsage{
		LegacyGenesis: 1250,
		Ecosystem:     100,
		Treasury:      50,
	}

	state, err :=
		usage.Remaining(policy)

	if err != nil {
		t.Fatal(err)
	}

	if state.EcosystemRemaining != 14_999_900 {
		t.Fatalf(
			"unexpected ecosystem remaining: %d",
			state.EcosystemRemaining,
		)
	}

	if state.TreasuryRemaining != 9_999_950 {
		t.Fatalf(
			"unexpected treasury remaining: %d",
			state.TreasuryRemaining,
		)
	}

	if state.TotalRemaining != 39_998_600 {
		t.Fatalf(
			"expected total remaining 39998600, got %d",
			state.TotalRemaining,
		)
	}
}

func TestReservedBudgetRejectsPoolOverflow(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	usage := ReservedUsage{
		Liquidity: 5_000_001,
	}

	if err := usage.Validate(
		policy,
	); err == nil {

		t.Fatal(
			"expected liquidity allocation overflow to fail",
		)
	}
}

func TestReservedBudgetRejectsAggregateOverflow(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	usage := ReservedUsage{
		LegacyGenesis: 39_000_000,
		Liquidity:     2_000_000,
	}

	if err := usage.Validate(
		policy,
	); err == nil {

		t.Fatal(
			"expected aggregate reserved allocation overflow to fail",
		)
	}
}
