package p2p

import (
	"strings"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func makePhase6WalletFixture(t *testing.T) (*Server, *wallet.Wallet, *wallet.Wallet) {
	t.Helper()
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.NewBlockchain(map[string]uint64{
		alice.Address: 100, bob.Address: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewServer("phase6-node", "127.0.0.1:7001", t.TempDir(), chain, nil, nil), alice, bob
}

// Sending a V1 payment must fail before attempting to dial the peer. An
// incoming V1 payment must also fail without altering the mempool.
func TestP2PV060LiveSubmissionRequiresV2(t *testing.T) {
	server, alice, bob := makePhase6WalletFixture(t)
	legacy := transaction.New(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex())
	if err := legacy.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := transaction.ValidateSigned(legacy); err != nil {
		t.Fatal(err)
	}

	if err := server.acceptTransaction(legacy); err == nil || !strings.Contains(err.Error(), "require V2") {
		t.Fatalf("incoming legacy V1 unexpectedly accepted: %v", err)
	}
	if got := server.MempoolCount(); got != 0 {
		t.Fatalf("rejected legacy V1 mutated mempool: count %d", got)
	}
	if err := server.SendTransaction("127.0.0.1:1", legacy); err == nil || !strings.Contains(err.Error(), "require V2") {
		t.Fatalf("outgoing legacy V1 was not rejected before TCP dial: %v", err)
	}
	chainID, err := server.Chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := transaction.NewV2(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex(), chainID, server.Chain.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := server.acceptTransaction(tx); err != nil {
		t.Fatalf("proper network-bound V2 rejected: %v", err)
	}
	if got := server.MempoolCount(); got != 1 {
		t.Fatalf("expected one valid V2 in mempool, got %d", got)
	}
}

func TestP2PV060IncomingForeignNetworkV2Rejected(t *testing.T) {
	server, alice, bob := makePhase6WalletFixture(t)
	chainID, err := server.Chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	foreign := "prism-ffffffffffffffff"
	if foreign == chainID {
		foreign = "prism-0000000000000000"
	}
	tx, err := transaction.NewV2(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex(), foreign, server.Chain.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := server.acceptTransaction(tx); err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("foreign V2 context accepted or wrong error: %v", err)
	}
	if server.MempoolCount() != 0 {
		t.Fatal("foreign V2 mutated mempool")
	}
}

// The V2 live-admission gate must never rewrite or invalidate historical
// blocks containing properly signed V1 transactions.
func TestP2PV060LegacyHistoryStillValid(t *testing.T) {
	server, block := makeBlockAcceptanceFixture(t)
	if len(block.Transactions) != 1 || block.Transactions[0].Version != 0 {
		t.Fatal("expected historical V1 fixture")
	}
	appended, err := server.acceptBlock(block)
	if err != nil {
		t.Fatalf("historical V1 block was rejected: %v", err)
	}
	if !appended {
		t.Fatal("historical block was not appended")
	}
	if !server.Chain.ValidateChain(server.PoS) {
		t.Fatal("chain invalid after historical V1 block acceptance")
	}
}
