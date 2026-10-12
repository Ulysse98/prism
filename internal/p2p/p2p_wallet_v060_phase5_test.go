package p2p

import (
	"strings"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

// The outgoing P2P path must reject valid signatures for another network
// without dialing a peer or waiting for the peer to detect the error.
func TestP2PV060OutgoingForeignNetworkRejectedBeforeDial(t *testing.T) {
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.NewBlockchain(map[string]uint64{alice.Address: 100, bob.Address: 100})
	if err != nil {
		t.Fatal(err)
	}
	correct, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	wrongID := "prism-ffffffffffffffff"
	if wrongID == correct {
		wrongID = "prism-0000000000000000"
	}
	server := NewServer("sender", "127.0.0.1:7001", t.TempDir(), chain, nil, nil)
	tx, err := transaction.NewV2(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex(), wrongID, chain.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := transaction.ValidateSigned(tx); err != nil {
		t.Fatalf("invalid fixture: %v", err)
	}
	err = server.SendTransaction("127.0.0.1:1", tx)
	if err == nil || !strings.Contains(err.Error(), "network mismatch") {
		t.Fatalf("expected local network rejection before TCP dial, got: %v", err)
	}

	wrongGenesis := strings.Repeat("f", 64)
	if wrongGenesis == chain.Blocks[0].Hash {
		wrongGenesis = strings.Repeat("0", 64)
	}
	tx2, err := transaction.NewV2(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex(), correct, wrongGenesis)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx2.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	err = server.SendTransaction("127.0.0.1:1", tx2)
	if err == nil || !strings.Contains(err.Error(), "network mismatch") {
		t.Fatalf("expected local genesis rejection before TCP dial, got: %v", err)
	}
}
