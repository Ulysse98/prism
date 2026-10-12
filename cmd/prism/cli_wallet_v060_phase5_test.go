package main

import (
	"testing"

	"prism/internal/storage"
	"prism/internal/transaction"
)

// Tests the real user-facing `send` command without changing the global
// testnet state: t.Chdir restores the original working directory afterward.
func TestWalletV060CLISendProducesV2(t *testing.T) {
	t.Chdir(t.TempDir())
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(dataDir, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}

	id, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	genesis := chain.Blocks[0].Hash
	if err := runSend("Alice", "Bob", "1", chain, pos, wallets); err != nil {
		t.Fatal(err)
	}
	stored, storedPos, _, err := storage.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.ValidateChain(storedPos) {
		t.Fatal("CLI produced invalid chain")
	}
	tip := stored.Blocks[len(stored.Blocks)-1]
	if len(tip.Transactions) != 1 {
		t.Fatalf("want 1 tx, got %d", len(tip.Transactions))
	}
	tx := tip.Transactions[0]
	if tx.Version != transaction.VersionV2 || tx.ChainID != id || tx.GenesisHash != genesis {
		t.Fatalf("CLI emitted non-network-bound transaction: %+v", tx)
	}
	if err := stored.ValidateTransactionContext(tx); err != nil {
		t.Fatal(err)
	}
}
