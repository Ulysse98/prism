package participation

import "testing"

func TestRewardClaimsAreDeterministic(
	t *testing.T,
) {
	book := NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Bob",
		1,
	); err != nil {
		t.Fatal(err)
	}

	if err := book.MarkClaimed(
		"Alice",
		2,
	); err != nil {
		t.Fatal(err)
	}

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err != nil {
		t.Fatal(err)
	}

	claims := book.Claims()

	if len(claims) != 3 {
		t.Fatalf(
			"expected 3 claims, got %d",
			len(claims),
		)
	}

	if claims[0].Address != "Alice" ||
		claims[0].Period != 0 {

		t.Fatalf(
			"unexpected first claim: %+v",
			claims[0],
		)
	}

	if claims[1].Address != "Alice" ||
		claims[1].Period != 2 {

		t.Fatalf(
			"unexpected second claim: %+v",
			claims[1],
		)
	}

	if claims[2].Address != "Bob" ||
		claims[2].Period != 1 {

		t.Fatalf(
			"unexpected third claim: %+v",
			claims[2],
		)
	}
}

func TestRewardClaimBookRestoresClaims(
	t *testing.T,
) {
	book, err :=
		NewRewardClaimBookFromClaims(
			[]RewardClaimKey{
				{
					Address: "Alice",
					Period:  0,
				},
				{
					Address: "Bob",
					Period:  1,
				},
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if !book.IsClaimed("Alice", 0) {
		t.Fatal(
			"expected restored Alice claim",
		)
	}

	if !book.IsClaimed("Bob", 1) {
		t.Fatal(
			"expected restored Bob claim",
		)
	}

	if book.Count() != 2 {
		t.Fatalf(
			"expected 2 restored claims, got %d",
			book.Count(),
		)
	}
}

func TestRewardClaimBookRestoreRejectsDuplicate(
	t *testing.T,
) {
	_, err :=
		NewRewardClaimBookFromClaims(
			[]RewardClaimKey{
				{
					Address: "Alice",
					Period:  0,
				},
				{
					Address: "Alice",
					Period:  0,
				},
			},
		)

	if err == nil {
		t.Fatal(
			"expected duplicate restored claim to fail",
		)
	}
}
