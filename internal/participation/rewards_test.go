package participation

import "testing"

func TestRewardUnitsBelowThreshold(
	t *testing.T,
) {
	if units := RewardUnits(9); units != 0 {
		t.Fatalf(
			"expected 0 reward units, got %d",
			units,
		)
	}
}

func TestRewardUnitsAtThreshold(
	t *testing.T,
) {
	if units := RewardUnits(10); units != 1 {
		t.Fatalf(
			"expected 1 reward unit, got %d",
			units,
		)
	}
}

func TestRewardUnitsUseWholeUnits(
	t *testing.T,
) {
	if units := RewardUnits(29); units != 2 {
		t.Fatalf(
			"expected 2 reward units, got %d",
			units,
		)
	}
}

func TestRewardUnitsAreCapped(
	t *testing.T,
) {
	if units := RewardUnits(1000); units != 10 {
		t.Fatalf(
			"expected capped reward units 10, got %d",
			units,
		)
	}
}
