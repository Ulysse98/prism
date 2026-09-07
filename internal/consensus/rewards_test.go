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

func TestRewardPolicyRejectsZeroReward(
	t *testing.T,
) {
	policy := RewardPolicy{
		ProposerReward:   0,
		UsefulWorkReward: 2,
	}

	if err := policy.Validate(); err == nil {
		t.Fatal(
			"expected zero proposer reward to fail validation",
		)
	}
}

func TestBlockEmissionRejectsOverflow(
	t *testing.T,
) {
	policy := RewardPolicy{
		ProposerReward:   1,
		UsefulWorkReward: math.MaxUint64,
	}

	if _, err := policy.BlockEmission(2); err == nil {
		t.Fatal(
			"expected useful work emission overflow",
		)
	}
}
