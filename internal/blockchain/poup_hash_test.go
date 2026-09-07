package blockchain

import (
	"testing"
	"time"

	"prism/internal/poup"
	"prism/internal/usefulwork"
)

func TestEmptyPoUPClaimsPreserveLegacyHash(
	t *testing.T,
) {
	block := Block{
		Height:       1,
		Timestamp:    time.Unix(1000, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "Alice",
		Reward:       BlockReward,
		Transactions: nil,
		UsefulWork:   []usefulwork.Proof{},
	}

	legacyHash := CalculateHash(block)

	block.ParticipationClaims =
		[]poup.Claim{}

	if got := CalculateHash(block); got != legacyHash {

		t.Fatal(
			"empty PoUP claims changed legacy block hash",
		)
	}
}

func TestPoUPClaimChangesBlockHash(
	t *testing.T,
) {
	block := Block{
		Height:       1,
		Timestamp:    time.Unix(1000, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "Alice",
		Reward:       BlockReward,
		Transactions: nil,
		UsefulWork:   []usefulwork.Proof{},
	}

	legacyHash := CalculateHash(block)

	block.ParticipationClaims =
		[]poup.Claim{
			{
				ID:      "claim-1",
				Address: "Alice",
				Period:  0,
				Points:  1200,
				Units:   10,
				Amount:  10,
			},
		}

	claimHash := CalculateHash(block)

	if claimHash == legacyHash {
		t.Fatal(
			"PoUP claim did not change block hash",
		)
	}

	block.ParticipationClaims[0].Amount = 9

	if CalculateHash(block) == claimHash {
		t.Fatal(
			"tampered PoUP claim did not change block hash",
		)
	}
}
