package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
)

func TestCalculateHashCommitsAuthorityChanges(t *testing.T) {
	timestamp := time.Date(
		2026,
		time.September,
		10,
		17,
		0,
		0,
		0,
		time.UTC,
	)

	changeA := reserved.NewAuthorityChange(
		"prism-test-chain",
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		"authority-a",
	)

	changeB := reserved.NewAuthorityChange(
		"prism-test-chain",
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		"authority-b",
	)

	blockA := Block{
		Height:       42,
		Timestamp:    timestamp,
		PreviousHash: "previous-hash",
		Proposer:     "validator-a",
		Reward:       3,
		Transactions: []transaction.Transaction{},
		UsefulWork:   []usefulwork.Proof{},
		AuthorityChanges: []reserved.AuthorityChange{
			changeA,
		},
	}

	blockB := blockA
	blockB.AuthorityChanges = []reserved.AuthorityChange{
		changeB,
	}

	hashA := CalculateHash(blockA)
	hashB := CalculateHash(blockB)

	if hashA == hashB {
		t.Fatal(
			"authority change mutation did not change block hash",
		)
	}
}

func TestCalculateHashCommitsAuthorityChangeNonce(t *testing.T) {
	timestamp := time.Date(
		2026,
		time.September,
		10,
		17,
		0,
		0,
		0,
		time.UTC,
	)

	changeA := reserved.NewAuthorityChange(
		"prism-test-chain",
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		"authority-a",
	)

	changeB := reserved.NewAuthorityChange(
		"prism-test-chain",
		2,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		"authority-a",
	)

	blockA := Block{
		Height:       42,
		Timestamp:    timestamp,
		PreviousHash: "previous-hash",
		Proposer:     "validator-a",
		Reward:       3,
		Transactions: []transaction.Transaction{},
		UsefulWork:   []usefulwork.Proof{},
		AuthorityChanges: []reserved.AuthorityChange{
			changeA,
		},
	}

	blockB := blockA
	blockB.AuthorityChanges = []reserved.AuthorityChange{
		changeB,
	}

	if CalculateHash(blockA) == CalculateHash(blockB) {
		t.Fatal(
			"authority change nonce mutation did not change block hash",
		)
	}
}

func TestCalculateHashCommitsAuthorityChangeAction(t *testing.T) {
	timestamp := time.Date(
		2026,
		time.September,
		10,
		17,
		0,
		0,
		0,
		time.UTC,
	)

	addChange := reserved.NewAuthorityChange(
		"prism-test-chain",
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		"authority-a",
	)

	removeChange := reserved.NewAuthorityChange(
		"prism-test-chain",
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeRemove,
		"authority-a",
	)

	blockA := Block{
		Height:       42,
		Timestamp:    timestamp,
		PreviousHash: "previous-hash",
		Proposer:     "validator-a",
		Reward:       3,
		Transactions: []transaction.Transaction{},
		UsefulWork:   []usefulwork.Proof{},
		AuthorityChanges: []reserved.AuthorityChange{
			addChange,
		},
	}

	blockB := blockA
	blockB.AuthorityChanges = []reserved.AuthorityChange{
		removeChange,
	}

	if CalculateHash(blockA) == CalculateHash(blockB) {
		t.Fatal(
			"authority change action mutation did not change block hash",
		)
	}
}

func TestCalculateHashWithoutAuthorityChangesRetainsLegacyHash(t *testing.T) {
	block := Block{
		Height: 42,
		Timestamp: time.Date(
			2026,
			time.September,
			10,
			17,
			0,
			0,
			0,
			time.UTC,
		),
		PreviousHash: "previous-hash",
		Proposer:     "validator-a",
		Reward:       3,
		Transactions: []transaction.Transaction{},
		UsefulWork:   []usefulwork.Proof{},
	}

	const expectedHash = "75aac8bb06fcdb91dc0c7cba87bcba7f6c8683bf927b61e64810822fe9819e03"

	got := CalculateHash(block)

	if got != expectedHash {
		t.Fatalf(
			"legacy block hash changed: expected=%s got=%s",
			expectedHash,
			got,
		)
	}
}
