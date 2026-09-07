package participation

import (
	"math"
	"testing"
)

func TestRewardClaimBookAcceptsFirstClaim(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err != nil {
		t.Fatal(err)
	}

	if !book.IsClaimed("Alice", 0) {
		t.Fatal(
			"expected Alice period 0 to be claimed",
		)
	}

	if book.Count() != 1 {
		t.Fatalf(
			"expected 1 claim, got %d",
			book.Count(),
		)
	}
}

func TestRewardClaimBookRejectsDuplicate(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err != nil {
		t.Fatal(err)
	}

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err == nil {
		t.Fatal(
			"expected duplicate claim to be rejected",
		)
	}

	if book.Count() != 1 {
		t.Fatalf(
			"duplicate claim mutated claim count: %d",
			book.Count(),
		)
	}
}

func TestRewardClaimBookAllowsDifferentPeriods(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err != nil {
		t.Fatal(err)
	}

	if err := book.MarkClaimed(
		"Alice",
		1,
	); err != nil {
		t.Fatal(err)
	}

	if book.Count() != 2 {
		t.Fatalf(
			"expected 2 claims, got %d",
			book.Count(),
		)
	}
}

func TestRewardClaimBookAllowsDifferentAddresses(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err != nil {
		t.Fatal(err)
	}

	if err := book.MarkClaimed(
		"Bob",
		0,
	); err != nil {
		t.Fatal(err)
	}

	if book.Count() != 2 {
		t.Fatalf(
			"expected 2 claims, got %d",
			book.Count(),
		)
	}
}

func TestRewardClaimBookRejectsEmptyAddress(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"",
		0,
	); err == nil {
		t.Fatal(
			"expected empty address to be rejected",
		)
	}
}

func TestRewardClaimBookRejectsInvalidPeriod(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Alice",
		math.MaxUint64,
	); err == nil {
		t.Fatal(
			"expected overflowing period to be rejected",
		)
	}
}
