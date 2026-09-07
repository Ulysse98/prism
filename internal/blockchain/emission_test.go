package blockchain

import (
	"testing"

	"prism/internal/poup"
)

func TestGenesisHasZeroNetworkEmission(
	t *testing.T,
) {
	chain, err :=
		NewBlockchain(
			map[string]uint64{
				"Alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	emission, err :=
		chain.GetEmissionState()

	if err != nil {
		t.Fatal(err)
	}

	if emission.ProposerEmission != 0 ||
		emission.UsefulWorkEmission != 0 ||
		emission.ParticipationEmission != 0 {

		t.Fatalf(
			"expected zero genesis network emission, got %+v",
			emission,
		)
	}

	total, err :=
		emission.NetworkEmission()

	if err != nil {
		t.Fatal(err)
	}

	if total != 0 {
		t.Fatalf(
			"expected zero network emission, got %d",
			total,
		)
	}
}

func TestEmissionStateTracksPoSPoUWAndPoUP(
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

	const (
		expectedPoS  uint64 = 505
		expectedPoUW uint64 = 200
		expectedPoUP uint64 = 10
	)

	if emission.ProposerEmission !=
		expectedPoS {

		t.Fatalf(
			"expected proposer emission %d, got %d",
			expectedPoS,
			emission.ProposerEmission,
		)
	}

	if emission.UsefulWorkEmission !=
		expectedPoUW {

		t.Fatalf(
			"expected useful work emission %d, got %d",
			expectedPoUW,
			emission.UsefulWorkEmission,
		)
	}

	if emission.ParticipationEmission !=
		expectedPoUP {

		t.Fatalf(
			"expected participation emission %d, got %d",
			expectedPoUP,
			emission.ParticipationEmission,
		)
	}

	total, err :=
		emission.NetworkEmission()

	if err != nil {
		t.Fatal(err)
	}

	if total != 715 {
		t.Fatalf(
			"expected network emission 715, got %d",
			total,
		)
	}
}
