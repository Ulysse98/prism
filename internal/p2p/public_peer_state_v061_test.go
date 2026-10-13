package p2p

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestV061PublicPeerPersistsValidChainWithoutPrivateWallets(t *testing.T) {
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.NewBlockchain(map[string]uint64{alice.Address: 100, bob.Address: 50})
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.LockStake(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	pos := consensus.NewProofOfStake()
	if err := pos.Register(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	if !chain.ValidateChain(pos) {
		t.Fatal("fixture chain invalid")
	}
	dir := t.TempDir()
	chainFile, err := json.Marshal(storage.ChainFile{Blockchain: chain, Validators: pos.Validators})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chain.json"), chainFile, 0600); err != nil {
		t.Fatal(err)
	}
	public, err := json.Marshal([]storage.PublicWalletRecord{{Name: "Alice", Address: alice.Address}, {Name: "Bob", Address: bob.Address}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api-wallets.json"), public, 0600); err != nil {
		t.Fatal(err)
	}
	tx := transaction.New(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex())
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if _, err := chain.AddBlock([]transaction.Transaction{tx}, nil, alice.Address, pos); err != nil {
		t.Fatal(err)
	}
	observers := map[string]*wallet.Wallet{
		"Alice": {Address: alice.Address},
		"Bob":   {Address: bob.Address},
	}
	if err := persistPeerStateV061(dir, chain, pos, observers); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wallets.json")); !os.IsNotExist(err) {
		t.Fatalf("private wallets.json created or stat failed: %v", err)
	}
	reloaded, _, _, err := storage.LoadPublic(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Blocks) != 2 || reloaded.Blocks[1].Hash != chain.Blocks[1].Hash {
		t.Fatal("public peer state not persisted correctly")
	}
}

func TestV061PublicPeerRejectsMixedState(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"wallets.json", "api-wallets.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("[]"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := persistPeerStateV061(dir, nil, nil, nil); err == nil {
		t.Fatal("mixed private/public wallet files must be rejected")
	}
}

func TestV061PublicPeerRejectsMissingState(t *testing.T) {
	if err := persistPeerStateV061(t.TempDir(), nil, nil, nil); err == nil {
		t.Fatal("missing state must be rejected")
	}
}
