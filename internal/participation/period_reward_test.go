package participation

import (
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

type neverEligible struct{}

func (neverEligible) IsVerifiedAtHeight(
	address string,
	height uint64,
) bool {
	return false
}

func TestEvaluatePeriodReward(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPeriodTestChain(
			t,
			101,
		)

	reward, err := EvaluatePeriodReward(
		chain,
		pos,
		alwaysEligible{},
		actor.Address,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if reward.Points != 1200 {
		t.Fatalf(
			"expected 1200 points, got %d",
			reward.Points,
		)
	}

	if reward.Units != 10 {
		t.Fatalf(
			"expected 10 reward units, got %d",
			reward.Units,
		)
	}

	if reward.Amount != 10 {
		t.Fatalf(
			"expected reward amount 10, got %d",
			reward.Amount,
		)
	}
}

func TestEvaluatePeriodRewardRejectsOpenPeriod(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPeriodTestChain(
			t,
			101,
		)

	if _, err := EvaluatePeriodReward(
		chain,
		pos,
		alwaysEligible{},
		actor.Address,
		1,
	); err == nil {
		t.Fatal(
			"expected incomplete period to be rejected",
		)
	}
}

func TestEvaluatePeriodRewardAllowsZeroReward(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPeriodTestChain(
			t,
			101,
		)

	reward, err := EvaluatePeriodReward(
		chain,
		pos,
		neverEligible{},
		actor.Address,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if reward.Points != 0 ||
		reward.Units != 0 ||
		reward.Amount != 0 {

		t.Fatalf(
			"expected zero reward, got %+v",
			reward,
		)
	}
}

func TestEvaluatePeriodRewardRejectsEmptyAddress(
	t *testing.T,
) {
	chain, pos, _ :=
		buildPeriodTestChain(
			t,
			101,
		)

	if _, err := EvaluatePeriodReward(
		chain,
		pos,
		alwaysEligible{},
		"",
		0,
	); err == nil {
		t.Fatal(
			"expected empty reward address to fail",
		)
	}
}

func TestBoundedParticipationRewardUsesFullAmount(
	t *testing.T,
) {
	amount, err :=
		boundedParticipationRewardAmount(
			10,
			blockchain.EmissionState{},
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if amount != 10 {
		t.Fatalf(
			"expected full PoUP reward 10, got %d",
			amount,
		)
	}
}

func TestBoundedParticipationRewardUsesFinalPartialAmount(
	t *testing.T,
) {
	amount, err :=
		boundedParticipationRewardAmount(
			10,
			blockchain.EmissionState{
				ParticipationEmission: 19_999_997,
			},
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if amount != 3 {
		t.Fatalf(
			"expected final PoUP reward 3, got %d",
			amount,
		)
	}
}

func TestBoundedParticipationRewardReturnsZeroWhenExhausted(
	t *testing.T,
) {
	amount, err :=
		boundedParticipationRewardAmount(
			10,
			blockchain.EmissionState{
				ParticipationEmission: 20_000_000,
			},
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if amount != 0 {
		t.Fatalf(
			"expected exhausted PoUP reward 0, got %d",
			amount,
		)
	}
}
