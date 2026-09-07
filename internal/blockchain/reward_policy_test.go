package blockchain

import (
	"testing"

	"prism/internal/consensus"
)

func TestLegacyRewardAliasesMatchConsensusPolicy(
	t *testing.T,
) {
	policy := consensus.DefaultRewardPolicy()

	if BlockReward != policy.ProposerReward {
		t.Fatalf(
			"block reward compatibility mismatch: blockchain=%d consensus=%d",
			BlockReward,
			policy.ProposerReward,
		)
	}

	if UsefulWorkReward != policy.UsefulWorkReward {
		t.Fatalf(
			"useful work reward compatibility mismatch: blockchain=%d consensus=%d",
			UsefulWorkReward,
			policy.UsefulWorkReward,
		)
	}
}
