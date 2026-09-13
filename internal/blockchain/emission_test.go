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

	// 101 non-genesis blocks:
	//
	// PoS:
	//   101 * 5 = 505
	//
	// PoUW:
	//   blocks 1-28   -> 28 * 2 = 56
	//   blocks 29-100 -> 72 * 1 = 72
	//   total         -> 128
	//
	// PoUP:
	//   one claim = 10
	const (
		expectedPoS  uint64 = 505
		expectedPoUW uint64 = 128
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

	const expectedNetworkEmission uint64 = expectedPoS +
		expectedPoUW +
		expectedPoUP

	if total != expectedNetworkEmission {
		t.Fatalf(
			"expected network emission %d, got %d",
			expectedNetworkEmission,
			total,
		)
	}
}
