package blockchain

import (
	"testing"

	"prism/internal/poup"
)

func TestAddParticipationClaimBlock(
	t *testing.T,
) {
	chain, pos, actor :=
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

	before, err :=
		chain.GetEmissionState()

	if err != nil {
		t.Fatal(err)
	}

	block, err :=
		chain.AddParticipationClaimBlock(
			[]poup.Claim{
				claim,
			},
			actor.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if block.Height != 101 {
		t.Fatalf(
			"expected claim block height 101, got %d",
			block.Height,
		)
	}

	if len(block.ParticipationClaims) != 1 {
		t.Fatalf(
			"expected one participation claim, got %d",
			len(block.ParticipationClaims),
		)
	}

	if block.ParticipationClaims[0].ID != claim.ID {
		t.Fatal(
			"produced block did not preserve claim",
		)
	}

	if !chain.ValidateChain(pos) {
		t.Fatal(
			"expected produced PoUP block to validate",
		)
	}

	after, err :=
		chain.GetEmissionState()

	if err != nil {
		t.Fatal(err)
	}

	expected :=
		before.ParticipationEmission +
			claim.Amount

	if after.ParticipationEmission != expected {
		t.Fatalf(
			"expected PoUP emission %d, got %d",
			expected,
			after.ParticipationEmission,
		)
	}
}

func TestAddParticipationClaimBlockRejectsEmptyClaims(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPoUPClaimValidationChain(
			t,
			1,
		)

	before :=
		len(chain.Blocks)

	if _, err :=
		chain.AddParticipationClaimBlock(
			nil,
			actor.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected empty PoUP block to fail",
		)
	}

	if len(chain.Blocks) != before {
		t.Fatal(
			"failed PoUP block mutated chain",
		)
	}
}

func TestAddParticipationClaimBlockRejectsInvalidClaim(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	// Period 0 deterministically earns 10 PRISM here,
	// so a signed amount of 9 must fail consensus.
	claim :=
		signedPoUPClaim(
			t,
			actor,
			1200,
			10,
			9,
		)

	before :=
		len(chain.Blocks)

	if _, err :=
		chain.AddParticipationClaimBlock(
			[]poup.Claim{
				claim,
			},
			actor.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected invalid PoUP claim block to fail",
		)
	}

	if len(chain.Blocks) != before {
		t.Fatal(
			"invalid PoUP claim block mutated chain",
		)
	}
}
