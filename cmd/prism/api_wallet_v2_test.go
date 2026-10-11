package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prism/internal/transaction"
)

func TestWalletV2SigningContextEndpoint(t *testing.T) {
	api, _, expectedChainID := walletTransferTestAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/context", nil)
	rr := httptest.NewRecorder()
	api.handleWalletSigningContext(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected signing context HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var payload struct {
		ChainID            string `json:"chainId"`
		GenesisHash        string `json:"genesisHash"`
		TransactionVersion uint8  `json:"transactionVersion"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ChainID != expectedChainID || payload.TransactionVersion != transaction.VersionV2 || len(payload.GenesisHash) != 64 {
		t.Fatalf("unexpected signing context %+v", payload)
	}
	bad := httptest.NewRecorder()
	api.handleWalletSigningContext(bad, httptest.NewRequest(http.MethodPost, "/api/v1/transactions/context", nil))
	if bad.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST context accepted: %d", bad.Code)
	}
}

func TestWalletTransferAPIRejectsLegacyV1(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	alice := wallets["Alice"]
	legacy := transaction.New(alice.Address, wallets["Bob"].Address, 1, 0, alice.PublicKeyHex())
	if err := legacy.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	resp := requestWalletTransfer(t, api, chainID, legacy)
	if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "version 2") {
		t.Fatalf("v1 wallet transfer accepted: %d %s", resp.Code, resp.Body.String())
	}
}

func TestWalletTransferAPIRejectsReSignedForeignNetwork(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	alice := wallets["Alice"]
	tx := transferTestTx(t, api, alice, wallets["Bob"].Address, 1, 0)
	tx.ChainID = "prism-ffffffffffffffff"
	if tx.ChainID == chainID {
		t.Fatal("test fixture accidentally uses own chain ID")
	}
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	resp := requestWalletTransfer(t, api, chainID, tx)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("foreign signed transaction accepted: %d %s", resp.Code, resp.Body.String())
	}
	tx = transferTestTx(t, api, alice, wallets["Bob"].Address, 1, 0)
	tx.GenesisHash = strings.Repeat("f", 64)
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	resp = requestWalletTransfer(t, api, chainID, tx)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("foreign genesis accepted: %d %s", resp.Code, resp.Body.String())
	}
}
