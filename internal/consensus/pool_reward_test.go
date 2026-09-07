package consensus

import "testing"

func TestBoundedPoolRewardUsesBaseReward(
	t *testing.T,
) {
	reward, err :=
		BoundedPoolReward(
			5,
			100,
			1000,
		)

	if err != nil {
		t.Fatal(err)
	}

	if reward != 5 {
		t.Fatalf(
			"expected reward 5, got %d",
			reward,
		)
	}
}

func TestBoundedPoolRewardUsesFinalPartialReward(
	t *testing.T,
) {
	reward, err :=
		BoundedPoolReward(
			5,
			19_999_997,
			20_000_000,
		)

	if err != nil {
		t.Fatal(err)
	}

	if reward != 3 {
		t.Fatalf(
			"expected final partial reward 3, got %d",
			reward,
		)
	}
}

func TestBoundedPoolRewardReturnsZeroWhenExhausted(
	t *testing.T,
) {
	reward, err :=
		BoundedPoolReward(
			5,
			20_000_000,
			20_000_000,
		)

	if err != nil {
		t.Fatal(err)
	}

	if reward != 0 {
		t.Fatalf(
			"expected exhausted reward 0, got %d",
			reward,
		)
	}
}

func TestBoundedPoolRewardRejectsExceededPool(
	t *testing.T,
) {
	if _, err :=
		BoundedPoolReward(
			5,
			20_000_001,
			20_000_000,
		); err == nil {

		t.Fatal(
			"expected exceeded pool to fail",
		)
	}
}

func TestBoundedPoolRewardUsesFinalPoUWUnit(
	t *testing.T,
) {
	reward, err :=
		BoundedPoolReward(
			2,
			19_999_999,
			20_000_000,
		)

	if err != nil {
		t.Fatal(err)
	}

	if reward != 1 {
		t.Fatalf(
			"expected final PoUW reward 1, got %d",
			reward,
		)
	}
}

func TestBoundedPoolRewardStopsPoUWWhenExhausted(
	t *testing.T,
) {
	reward, err :=
		BoundedPoolReward(
			2,
			20_000_000,
			20_000_000,
		)

	if err != nil {
		t.Fatal(err)
	}

	if reward != 0 {
		t.Fatalf(
			"expected exhausted PoUW reward 0, got %d",
			reward,
		)
	}
}
