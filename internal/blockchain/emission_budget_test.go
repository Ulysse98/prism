package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/poup"
)

func TestEmissionStateReportsRemainingRewardPools(
	t *testing.T,
) {
	chain, _, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	claim :=
		signedPoUPClaim(
			t,
			actor,
			1200,
			10,
			10,
		)

	appendPoUPClaimBlock(
		t,
		chain,
		actor,
		[]poup.Claim{
			claim,
		},
	)

	emission, err :=
		chain.GetEmissionState()

	if err != nil {
		t.Fatal(err)
	}

	remaining, err :=
		emission.RemainingRewardPools(
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if remaining.ProposerRemaining !=
		19_999_495 {

		t.Fatalf(
			"expected proposer remaining 19999495, got %d",
			remaining.ProposerRemaining,
		)
	}

	if remaining.UsefulWorkRemaining !=
		19_999_800 {

		t.Fatalf(
			"expected useful work remaining 19999800, got %d",
			remaining.UsefulWorkRemaining,
		)
	}

	if remaining.ParticipationRemaining !=
		19_999_990 {

		t.Fatalf(
			"expected participation remaining 19999990, got %d",
			remaining.ParticipationRemaining,
		)
	}

	total, err :=
		remaining.NetworkRemaining()

	if err != nil {
		t.Fatal(err)
	}

	if total != 59_999_285 {
		t.Fatalf(
			"expected remaining network rewards 59999285, got %d",
			total,
		)
	}
}

func TestRemainingRewardPoolsRejectsExceededPool(
	t *testing.T,
) {
	policy :=
		consensus.DefaultSupplyPolicy()

	emission := EmissionState{
		ProposerEmission: policy.ProposerRewardPool + 1,
	}

	if _, err :=
		emission.RemainingRewardPools(
			policy,
		); err == nil {

		t.Fatal(
			"expected exceeded proposer reward pool to fail",
		)
	}
}

func TestRemainingRewardPoolsAllowsExactExhaustion(
	t *testing.T,
) {
	policy :=
		consensus.DefaultSupplyPolicy()

	emission := EmissionState{
		ProposerEmission: policy.ProposerRewardPool,
	}

	remaining, err :=
		emission.RemainingRewardPools(
			policy,
		)

	if err != nil {
		t.Fatal(err)
	}

	if remaining.ProposerRemaining != 0 {
		t.Fatalf(
			"expected exhausted proposer pool to have zero remaining, got %d",
			remaining.ProposerRemaining,
		)
	}
}
