package participation

import (
	"fmt"
	"sort"
)

func (book *RewardClaimBook) Claims() []RewardClaimKey {
	if book == nil {
		return []RewardClaimKey{}
	}

	book.mu.RLock()
	defer book.mu.RUnlock()

	claims := make(
		[]RewardClaimKey,
		0,
		len(book.claimed),
	)

	for key := range book.claimed {
		claims = append(
			claims,
			key,
		)
	}

	sort.Slice(
		claims,
		func(i, j int) bool {
			if claims[i].Address ==
				claims[j].Address {

				return claims[i].Period <
					claims[j].Period
			}

			return claims[i].Address <
				claims[j].Address
		},
	)

	return claims
}

func NewRewardClaimBookFromClaims(
	claims []RewardClaimKey,
) (*RewardClaimBook, error) {
	book := NewRewardClaimBook()

	for _, claim := range claims {
		if err := book.MarkClaimed(
			claim.Address,
			claim.Period,
		); err != nil {

			return nil, fmt.Errorf(
				"invalid reward claim %s period %d: %w",
				claim.Address,
				claim.Period,
				err,
			)
		}
	}

	return book, nil
}
