package blockchain

import (
	"math"
	"testing"

	"prism/internal/consensus"
	"prism/internal/poup"
)

func TestPoUPClaimIncreasesTotalSupply(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPoUPClaimValidationChain(
			t,
			100,
		)

	claim := signedPoUPClaim(
		t,
		actor,
		1200,
		10,
		10,
	)

	before, err :=
		chain.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}

	appendPoUPClaimBlock(
		t,
		chain,
		actor,
		[]poup.Claim{
			claim,
		},
	)

	if !chain.ValidateChain(pos) {
		t.Fatal(
			"expected chain with PoUP mint to validate",
		)
	}

	after, err :=
		chain.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}

	expected :=
		before +
			consensus.DefaultRewardPolicy().
				ProposerReward +
			claim.Amount

	if after != expected {
		t.Fatalf(
			"expected total supply %d, got %d",
			expected,
			after,
		)
	}
}

func TestCreditParticipationRewardRejectsOverflow(
	t *testing.T,
) {
	state := State{
		Balances: map[string]uint64{
			"Alice": math.MaxUint64,
		},
		Nonces: make(
			map[string]uint64,
		),
	}

	claim := poup.Claim{
		Address: "Alice",
		Amount:  1,
	}

	if err := creditParticipationReward(
		&state,
		claim,
	); err == nil {
		t.Fatal(
			"expected PoUP balance overflow to fail",
		)
	}

	if state.Balances["Alice"] !=
		math.MaxUint64 {

		t.Fatal(
			"overflow attempt mutated balance",
		)
	}
}
