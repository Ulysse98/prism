package transaction

import (
	"encoding/json"
	"strings"
	"testing"

	"prism/internal/wallet"
)

func v2Fixture(t *testing.T) (*wallet.Wallet, *wallet.Wallet, Transaction) {
	t.Helper()
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := NewV2(alice.Address, bob.Address, 25, 0, alice.PublicKeyHex(),
		"prism-1234567890abcdef", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	return alice, bob, tx
}

func TestV2SignatureBindsNetworkAndTransfer(t *testing.T) {
	_, _, tx := v2Fixture(t)
	if err := ValidateOnChain(tx, tx.ChainID, tx.GenesisHash); err != nil {
		t.Fatalf("valid v2 transaction rejected: %v", err)
	}
	for _, changed := range []Transaction{
		func() Transaction { cp := tx; cp.ChainID = "prism-eeeeeeeeeeeeeeee"; return cp }(),
		func() Transaction { cp := tx; cp.GenesisHash = strings.Repeat("b", 64); return cp }(),
		func() Transaction { cp := tx; cp.Amount++; return cp }(),
		func() Transaction { cp := tx; cp.Nonce++; return cp }(),
		func() Transaction { cp := tx; cp.Version = 0; return cp }(),
	} {
		if ValidateSigned(changed) == nil {
			t.Fatal("modified signed field accepted")
		}
	}
	if err := ValidateOnChain(tx, "prism-eeeeeeeeeeeeeeee", tx.GenesisHash); err == nil {
		t.Fatal("cross-chain replay accepted")
	}
	if err := ValidateOnChain(tx, tx.ChainID, strings.Repeat("b", 64)); err == nil {
		t.Fatal("cross-genesis replay accepted")
	}
}

func TestV2SignatureIsValidOnlyWhenResignedForNewContext(t *testing.T) {
	alice, _, tx := v2Fixture(t)
	other := tx
	other.ChainID = "prism-eeeeeeeeeeeeeeee"
	if err := other.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if other.ID == tx.ID {
		t.Fatal("cross-network transaction IDs must differ")
	}
	if err := ValidateOnChain(other, tx.ChainID, tx.GenesisHash); err == nil {
		t.Fatal("independently signed foreign transaction accepted on home network")
	}
}

func TestLegacyWireFormatAndSignatureUnchanged(t *testing.T) {
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	legacy := New(alice.Address, bob.Address, 2, 0, alice.PublicKeyHex())
	if err := legacy.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSigned(legacy); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"version"`, `"chain_id"`, `"genesis_hash"`} {
		if strings.Contains(string(encoded), field) {
			t.Fatalf("v1 JSON wire changed: %s", encoded)
		}
	}
	withContext := legacy
	withContext.ChainID = "prism-1234567890abcdef"
	if ValidateSigned(withContext) == nil {
		t.Fatal("legacy network fields accepted")
	}
	unsupported := legacy
	unsupported.Version = 1
	if ValidateSigned(unsupported) == nil {
		t.Fatal("unknown transaction version accepted")
	}
}

func TestV2RequiresCanonicalAddressAndContext(t *testing.T) {
	alice, bob, _ := v2Fixture(t)
	for _, tc := range []struct{ to, chain, genesis string }{
		{bob.Address, "wrong-chain", strings.Repeat("a", 64)},
		{bob.Address, "prism-1234567890abcdef", "not-a-hash"},
		{"alice|bob", "prism-1234567890abcdef", strings.Repeat("a", 64)},
	} {
		if _, err := NewV2(alice.Address, tc.to, 1, 0, alice.PublicKeyHex(), tc.chain, tc.genesis); err == nil {
			t.Fatalf("invalid v2 context accepted: %+v", tc)
		}
	}
}
