package poup

import "testing"

func TestRewardPeriodBoundaries(
	t *testing.T,
) {
	tests := []struct {
		height uint64
		index  uint64
		start  uint64
		end    uint64
	}{
		{1, 0, 1, 100},
		{100, 0, 1, 100},
		{101, 1, 101, 200},
	}

	for _, test := range tests {
		period, err :=
			PeriodForHeight(
				test.height,
			)

		if err != nil {
			t.Fatal(err)
		}

		if period.Index != test.index ||
			period.StartHeight != test.start ||
			period.EndHeight != test.end {

			t.Fatalf(
				"unexpected period for height %d: %+v",
				test.height,
				period,
			)
		}
	}
}

func TestGenesisHasNoRewardPeriod(
	t *testing.T,
) {
	if _, err :=
		PeriodForHeight(0); err == nil {

		t.Fatal(
			"expected genesis period lookup to fail",
		)
	}
}

func TestRewardUnitsPolicy(
	t *testing.T,
) {
	tests := []struct {
		points uint64
		units  uint64
	}{
		{9, 0},
		{10, 1},
		{29, 2},
		{1000, 10},
	}

	for _, test := range tests {
		got := RewardUnits(
			test.points,
		)

		if got != test.units {
			t.Fatalf(
				"RewardUnits(%d): expected %d, got %d",
				test.points,
				test.units,
				got,
			)
		}
	}
}

func TestPoUPConsensusDefaults(
	t *testing.T,
) {
	if ProposerPoints != 10 {
		t.Fatalf(
			"unexpected proposer points: %d",
			ProposerPoints,
		)
	}

	if UsefulWorkPointMultiple != 2 {
		t.Fatalf(
			"unexpected useful work multiplier: %d",
			UsefulWorkPointMultiple,
		)
	}

	if RewardPeriodBlocks != 100 {
		t.Fatalf(
			"unexpected reward period size: %d",
			RewardPeriodBlocks,
		)
	}
}
