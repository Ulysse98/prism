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

	const (
		expectedProposerRemaining      uint64 = 19_999_495
		expectedUsefulWorkRemaining    uint64 = 19_999_872
		expectedParticipationRemaining uint64 = 19_999_990
	)

	if remaining.ProposerRemaining !=
		expectedProposerRemaining {

		t.Fatalf(
			"expected proposer remaining %d, got %d",
			expectedProposerRemaining,
			remaining.ProposerRemaining,
		)
	}

	if remaining.UsefulWorkRemaining !=
		expectedUsefulWorkRemaining {

		t.Fatalf(
			"expected useful work remaining %d, got %d",
			expectedUsefulWorkRemaining,
			remaining.UsefulWorkRemaining,
		)
	}

	if remaining.ParticipationRemaining !=
		expectedParticipationRemaining {

		t.Fatalf(
			"expected participation remaining %d, got %d",
			expectedParticipationRemaining,
			remaining.ParticipationRemaining,
		)
	}

	total, err :=
		remaining.NetworkRemaining()

	if err != nil {
		t.Fatal(err)
	}

	const expectedNetworkRemaining uint64 = expectedProposerRemaining +
		expectedUsefulWorkRemaining +
		expectedParticipationRemaining

	if total != expectedNetworkRemaining {
		t.Fatalf(
			"expected remaining network rewards %d, got %d",
			expectedNetworkRemaining,
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
