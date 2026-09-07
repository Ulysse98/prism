package consensus

import (
	"math"
	"testing"
)

func TestDefaultRewardPolicy(t *testing.T) {
	policy := DefaultRewardPolicy()

	if policy.ProposerReward != 5 {
		t.Fatalf(
			"expected proposer reward 5, got %d",
			policy.ProposerReward,
		)
	}

	if policy.UsefulWorkReward != 2 {
		t.Fatalf(
			"expected useful work reward 2, got %d",
			policy.UsefulWorkReward,
		)
	}

	if policy.ParticipationReward != 1 {
		t.Fatalf(
			"expected participation reward 1, got %d",
			policy.ParticipationReward,
		)
	}

	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBlockEmission(t *testing.T) {
	policy := DefaultRewardPolicy()

	emission, err := policy.BlockEmission(3)
	if err != nil {
		t.Fatal(err)
	}

	if emission != 11 {
		t.Fatalf(
			"expected block emission 11, got %d",
			emission,
		)
	}
}

func TestBlockEmissionWithoutUsefulWork(
	t *testing.T,
) {
	policy := DefaultRewardPolicy()

	emission, err := policy.BlockEmission(0)
	if err != nil {
		t.Fatal(err)
	}

	if emission != policy.ProposerReward {
		t.Fatalf(
			"expected proposer-only emission %d, got %d",
			policy.ProposerReward,
			emission,
		)
	}
}

func TestBlockEmissionWithParticipation(
	t *testing.T,
) {
	policy := DefaultRewardPolicy()

	emission, err :=
		policy.BlockEmissionWithParticipation(
			3,
			4,
		)

	if err != nil {
		t.Fatal(err)
	}

	// 5 proposer + 6 useful work + 4 participation.
	if emission != 15 {
		t.Fatalf(
			"expected total emission 15, got %d",
			emission,
		)
	}
}

func TestRewardPolicyRejectsZeroProposerReward(
	t *testing.T,
) {
	policy := RewardPolicy{
		ProposerReward:      0,
		UsefulWorkReward:    2,
		ParticipationReward: 1,
	}

	if err := policy.Validate(); err == nil {
		t.Fatal(
			"expected zero proposer reward to fail validation",
		)
	}
}

func TestRewardPolicyRejectsZeroParticipationReward(
	t *testing.T,
) {
	policy := RewardPolicy{
		ProposerReward:      5,
		UsefulWorkReward:    2,
		ParticipationReward: 0,
	}

	if err := policy.Validate(); err == nil {
		t.Fatal(
			"expected zero participation reward to fail validation",
		)
	}
}

func TestBlockEmissionRejectsUsefulWorkOverflow(
	t *testing.T,
) {
	policy := RewardPolicy{
		ProposerReward:      1,
		UsefulWorkReward:    math.MaxUint64,
		ParticipationReward: 1,
	}

	if _, err :=
		policy.BlockEmissionWithParticipation(
			2,
			0,
		); err == nil {

		t.Fatal(
			"expected useful work emission overflow",
		)
	}
}

func TestParticipationEmissionRejectsOverflow(
	t *testing.T,
) {
	policy := RewardPolicy{
		ProposerReward:      1,
		UsefulWorkReward:    1,
		ParticipationReward: math.MaxUint64,
	}

	if _, err :=
		policy.BlockEmissionWithParticipation(
			0,
			2,
		); err == nil {

		t.Fatal(
			"expected participation emission overflow",
		)
	}
}
