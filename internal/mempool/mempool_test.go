package mempool

import (
	"testing"

	"prism/internal/transaction"
)

func TestRemoveCommittedRemovesOnlyIncludedTransactions(
	t *testing.T,
) {
	pool := New()

	tx1 := transaction.Transaction{ID: "tx-1"}
	tx2 := transaction.Transaction{ID: "tx-2"}
	tx3 := transaction.Transaction{ID: "tx-3"}

	pool.transactions = []transaction.Transaction{
		tx1,
		tx2,
		tx3,
	}

	pool.ids = map[string]struct{}{
		tx1.ID: {},
		tx2.ID: {},
		tx3.ID: {},
	}

	pool.RemoveCommitted(
		[]transaction.Transaction{
			tx1,
			tx3,
		},
	)

	if pool.Count() != 1 {
		t.Fatalf(
			"expected 1 remaining transaction, got %d",
			pool.Count(),
		)
	}

	if pool.Has(tx1.ID) {
		t.Fatal(
			"expected committed tx-1 to be removed",
		)
	}

	if pool.Has(tx3.ID) {
		t.Fatal(
			"expected committed tx-3 to be removed",
		)
	}

	if !pool.Has(tx2.ID) {
		t.Fatal(
			"expected uncommitted tx-2 to remain",
		)
	}

	remaining := pool.Transactions()

	if len(remaining) != 1 ||
		remaining[0].ID != tx2.ID {

		t.Fatal(
			"unexpected remaining mempool contents",
		)
	}
}
