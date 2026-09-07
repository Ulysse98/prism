package participation

import (
	"fmt"
	"sync"
)

type RewardClaimKey struct {
	Address string
	Period  uint64
}

type RewardClaimBook struct {
	mu      sync.RWMutex
	claimed map[RewardClaimKey]struct{}
}

func NewRewardClaimBook() *RewardClaimBook {
	return &RewardClaimBook{
		claimed: make(
			map[RewardClaimKey]struct{},
		),
	}
}

func (book *RewardClaimBook) IsClaimed(
	address string,
	period uint64,
) bool {
	if book == nil {
		return false
	}

	key := RewardClaimKey{
		Address: address,
		Period:  period,
	}

	book.mu.RLock()
	defer book.mu.RUnlock()

	_, exists := book.claimed[key]

	return exists
}

func (book *RewardClaimBook) MarkClaimed(
	address string,
	period uint64,
) error {
	if book == nil {
		return fmt.Errorf(
			"reward claim book cannot be nil",
		)
	}

	if address == "" {
		return fmt.Errorf(
			"reward claim address cannot be empty",
		)
	}

	if _, err := PeriodByIndex(period); err != nil {
		return fmt.Errorf(
			"invalid reward claim period: %w",
			err,
		)
	}

	key := RewardClaimKey{
		Address: address,
		Period:  period,
	}

	book.mu.Lock()
	defer book.mu.Unlock()

	if book.claimed == nil {
		book.claimed = make(
			map[RewardClaimKey]struct{},
		)
	}

	if _, exists := book.claimed[key]; exists {
		return fmt.Errorf(
			"reward already claimed for address %s period %d",
			address,
			period,
		)
	}

	book.claimed[key] = struct{}{}

	return nil
}

func (book *RewardClaimBook) Count() int {
	if book == nil {
		return 0
	}

	book.mu.RLock()
	defer book.mu.RUnlock()

	return len(book.claimed)
}
