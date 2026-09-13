package blockchain

import (
	"testing"

	"prism/internal/consensus"
)

func TestPoUPClaimUsesFinalPartialPoolReward(
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
			3,
		)

	err :=
		chain.validateParticipationClaimWithEmission(
			claim,
			101,
			consensus.DefaultRewardPolicy(),
			19_999_997,
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}
}

func TestPoUPClaimRejectsUnboundedFinalAmount(
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

	err :=
		chain.validateParticipationClaimWithEmission(
			claim,
			101,
			consensus.DefaultRewardPolicy(),
			19_999_997,
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected unbounded PoUP amount to fail",
		)
	}
}

func TestPoUPClaimRejectsExhaustedPool(
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
			1,
		)

	err :=
		chain.validateParticipationClaimWithEmission(
			claim,
			101,
			consensus.DefaultRewardPolicy(),
			20_000_000,
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected exhausted PoUP pool to reject claim",
		)
	}
}
