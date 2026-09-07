package consensus

import "testing"

func TestDefaultSupplyPolicy(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}

	if policy.MaxSupply != 100_000_000 {
		t.Fatalf(
			"expected max supply 100000000, got %d",
			policy.MaxSupply,
		)
	}

	if policy.NetworkRewardAllocation() !=
		60_000_000 {

		t.Fatalf(
			"expected 60000000 network rewards, got %d",
			policy.NetworkRewardAllocation(),
		)
	}

	if policy.ReservedAllocation() !=
		40_000_000 {

		t.Fatalf(
			"expected 40000000 reserved allocation, got %d",
			policy.ReservedAllocation(),
		)
	}
}

func TestSupplyPolicyRewardPools(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	if policy.ProposerRewardPool != 20_000_000 {
		t.Fatal(
			"unexpected proposer reward pool",
		)
	}

	if policy.UsefulWorkRewardPool != 20_000_000 {
		t.Fatal(
			"unexpected useful work reward pool",
		)
	}

	if policy.ParticipationRewardPool !=
		20_000_000 {

		t.Fatal(
			"unexpected participation reward pool",
		)
	}
}

func TestSupplyPolicyReservedAllocations(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	if policy.EcosystemAllocation != 15_000_000 ||
		policy.TreasuryAllocation != 10_000_000 ||
		policy.TeamAllocation != 10_000_000 ||
		policy.LiquidityAllocation != 5_000_000 {

		t.Fatal(
			"unexpected reserved supply allocation",
		)
	}
}

func TestSupplyPolicyRejectsMismatch(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	policy.TeamAllocation--

	if err := policy.Validate(); err == nil {
		t.Fatal(
			"expected mismatched supply allocation to fail",
		)
	}
}
