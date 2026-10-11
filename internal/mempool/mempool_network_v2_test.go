package mempool

import (
	"testing"

	"prism/internal/blockchain"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestV2MempoolRejectsOtherNetworkAndAcceptsOwn(t *testing.T) {
	a, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	b, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.NewBlockchain(map[string]uint64{a.Address: 100})
	if err != nil {
		t.Fatal(err)
	}
	chainID, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := transaction.NewV2(a.Address, b.Address, 25, 0, a.PublicKeyHex(), chainID, chain.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Sign(a.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := New().Add(tx, chain); err != nil {
		t.Fatalf("own-network v2 rejected: %v", err)
	}

	otherChain, err := blockchain.NewBlockchain(map[string]uint64{a.Address: 200})
	if err != nil {
		t.Fatal(err)
	}
	if err := New().Add(tx, otherChain); err == nil {
		t.Fatal("other-network signed transaction accepted")
	}
}
