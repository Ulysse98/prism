package consensus

import "testing"

func TestUsefulWorkBaseRewardPreservesLegacyHistory(
	t *testing.T,
) {
	policy := DefaultRewardPolicy()

	reward := UsefulWorkBaseReward(
		28,
		12,
		policy,
	)

	if reward != 2 {
		t.Fatalf(
			"expected legacy reward 2, got %d",
			reward,
		)
	}
}

func TestUsefulWorkBaseRewardAfterActivation(
	t *testing.T,
) {
	policy := DefaultRewardPolicy()

	tests := []struct {
		name      string
		workUnits uint64
		expected  uint64
	}{
		{
			name:      "zero",
			workUnits: 0,
			expected:  0,
		},
		{
			name:      "low",
			workUnits: 3,
			expected:  1,
		},
		{
			name:      "medium lower",
			workUnits: 6,
			expected:  2,
		},
		{
			name:      "medium upper",
			workUnits: 8,
			expected:  2,
		},
		{
			name:      "high",
			workUnits: 12,
			expected:  3,
		},
		{
			name:      "very high",
			workUnits: 17,
			expected:  4,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				reward :=
					UsefulWorkBaseReward(
						UsefulWorkScoreRewardActivationHeight,
						test.workUnits,
						policy,
					)

				if reward != test.expected {
					t.Fatalf(
						"expected reward %d, got %d",
						test.expected,
						reward,
					)
				}
			},
		)
	}
}
