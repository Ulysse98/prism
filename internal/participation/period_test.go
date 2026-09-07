package participation

import "testing"

func TestPeriodForFirstBlock(
	t *testing.T,
) {
	period, err := PeriodForHeight(1)
	if err != nil {
		t.Fatal(err)
	}

	if period.Index != 0 ||
		period.StartHeight != 1 ||
		period.EndHeight != 100 {

		t.Fatalf(
			"unexpected first reward period: %+v",
			period,
		)
	}
}

func TestPeriodBoundary(
	t *testing.T,
) {
	first, err := PeriodForHeight(100)
	if err != nil {
		t.Fatal(err)
	}

	second, err := PeriodForHeight(101)
	if err != nil {
		t.Fatal(err)
	}

	if first.Index != 0 {
		t.Fatalf(
			"expected height 100 in period 0, got %d",
			first.Index,
		)
	}

	if second.Index != 1 {
		t.Fatalf(
			"expected height 101 in period 1, got %d",
			second.Index,
		)
	}
}

func TestPeriodRejectsGenesis(
	t *testing.T,
) {
	if _, err := PeriodForHeight(0); err == nil {
		t.Fatal(
			"expected genesis height to be rejected",
		)
	}
}

func TestPeriodByIndex(
	t *testing.T,
) {
	period, err := PeriodByIndex(2)
	if err != nil {
		t.Fatal(err)
	}

	if period.StartHeight != 201 ||
		period.EndHeight != 300 {

		t.Fatalf(
			"unexpected period bounds: %+v",
			period,
		)
	}
}
