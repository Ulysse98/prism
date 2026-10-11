package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func walletTransferTestAPI(t *testing.T) (*apiServer, map[string]*wallet.Wallet, string) {
	t.Helper()
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := storage.Save(dir, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}
	return &apiServer{
		dataPath: dir, computeMarket: compute.NewMarketplace(),
		walletTransfersEnabled: true, faucetEnabled: true,
		faucetWallet: "Alice", faucetAmount: 100,
	}, wallets, blockchain.MakeChainID(chain.Blocks[0].Hash)
}

func transferTestTx(t *testing.T, sender *wallet.Wallet, to string, amount, nonce uint64) transaction.Transaction {
	t.Helper()
	tx := transaction.New(sender.Address, to, amount, nonce, sender.PublicKeyHex())
	if err := tx.Sign(sender.PrivateKey); err != nil {
		t.Fatal(err)
	}
	return tx
}

func requestWalletTransfer(t *testing.T, api *apiServer, chainID string, tx transaction.Transaction) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(apiSignedTransferRequest{ChainID: chainID, Transaction: tx})
	if err != nil {
		t.Fatal(err)
	}
	return postRawWalletTransfer(api, body)
}

func postRawWalletTransfer(api *apiServer, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader(body))
	response := httptest.NewRecorder()
	api.handleWalletTransactions(response, request)
	return response
}

func getWalletTransferHistory(api *apiServer, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	api.handleWalletTransactions(response, request)
	return response
}

func TestSignedWalletTransferFromExternalWallet(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	external, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	if _, local := wallets[external.Address]; local {
		t.Fatal("wallet unexpectedly local")
	}

	// Fund the external address without giving its private key to the API.
	if _, code, err := api.dispenseFaucet(external.Address); err != nil || code != http.StatusCreated {
		t.Fatalf("fund external wallet: %v (HTTP %d)", err, code)
	}
	tx := transferTestTx(t, external, wallets["Bob"].Address, 25, 0)
	response := requestWalletTransfer(t, api, chainID, tx)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201, got %d: %s", response.Code, response.Body.String())
	}
	var result apiSignedTransferResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.TxID != tx.ID || result.From != external.Address || result.To != tx.To || result.Amount != 25 || result.Status != "locally_included" || result.ChainID != chainID {
		t.Fatalf("unexpected transfer receipt: %+v", result)
	}
	if strings.Contains(response.Body.String(), external.PublicKeyHex()) || strings.Contains(response.Body.String(), "private_key") {
		t.Fatal("transfer response must not leak wallet key material")
	}

	updated, _, _, err := storage.Load(api.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := updated.BalanceOf(external.Address)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 75 {
		t.Fatalf("expected external balance 75, got %d", balance)
	}
	nonce, err := updated.NonceOf(external.Address)
	if err != nil || nonce != 1 {
		t.Fatalf("expected nonce 1, got %d, err %v", nonce, err)
	}

	history := getWalletTransferHistory(api, "/api/v1/transactions?address="+external.Address)
	if history.Code != http.StatusOK {
		t.Fatalf("history HTTP %d: %s", history.Code, history.Body.String())
	}
	var decoded struct {
		Address string                  `json:"address"`
		Count   int                     `json:"count"`
		Entries []apiWalletHistoryEntry `json:"entries"`
	}
	if err := json.Unmarshal(history.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Address != external.Address || decoded.Count != 2 || len(decoded.Entries) != 2 {
		t.Fatalf("expected faucet + external spend in history, got %+v", decoded)
	}
	if decoded.Entries[0].TxID != tx.ID || decoded.Entries[0].Status != "locally_included" {
		t.Fatalf("wrong latest transaction: %+v", decoded.Entries[0])
	}
}

func TestSignedWalletTransferReplayAndInvalidSignature(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	alice, bob := wallets["Alice"], wallets["Bob"]
	tx := transferTestTx(t, alice, bob.Address, 1, 0)
	tampered := tx
	tampered.Signature = strings.Repeat("0", len(tx.Signature))
	if response := requestWalletTransfer(t, api, chainID, tampered); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid signature accepted: HTTP %d: %s", response.Code, response.Body.String())
	}
	if response := requestWalletTransfer(t, api, "other-network", tx); response.Code != http.StatusBadRequest {
		t.Fatalf("wrong chain ID accepted: HTTP %d", response.Code)
	}
	badRecipient := transferTestTx(t, alice, "not-a-prism-address", 1, 0)
	if response := requestWalletTransfer(t, api, chainID, badRecipient); response.Code != http.StatusBadRequest {
		t.Fatalf("bad recipient accepted: HTTP %d", response.Code)
	}
	if response := requestWalletTransfer(t, api, chainID, tx); response.Code != http.StatusCreated {
		t.Fatalf("valid tx rejected: HTTP %d: %s", response.Code, response.Body.String())
	}
	if response := requestWalletTransfer(t, api, chainID, tx); response.Code != http.StatusConflict {
		t.Fatalf("replay accepted: HTTP %d: %s", response.Code, response.Body.String())
	}
}

func TestSignedWalletTransferRequiresOptInAndStrictJSON(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	tx := transferTestTx(t, wallets["Alice"], wallets["Bob"].Address, 1, 0)
	api.walletTransfersEnabled = false
	if response := requestWalletTransfer(t, api, chainID, tx); response.Code != http.StatusForbidden {
		t.Fatalf("disabled transfers accepted: HTTP %d", response.Code)
	}
	api.walletTransfersEnabled = true
	for _, raw := range [][]byte{
		[]byte(`{"unknownField":3}`),
		[]byte(`{"chainId":"x", "transaction": {}} {}`),
		bytes.Repeat([]byte("x"), 8193),
	} {
		if response := postRawWalletTransfer(api, raw); response.Code != http.StatusBadRequest {
			t.Fatalf("bad JSON accepted: HTTP %d", response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/transactions", nil)
	response := httptest.NewRecorder()
	api.handleWalletTransactions(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT accepted: HTTP %d", response.Code)
	}
}

func TestSignedWalletTransferPreservesComputeReservations(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	alice := wallets["Alice"]
	chain, _, _, err := storage.Load(api.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	available, err := chain.AvailableBalanceOf(alice.Address)
	if err != nil || available < 12 {
		t.Fatalf("bad test balance: %d, %v", available, err)
	}
	task, err := usefulwork.NewSumSquaresTask([]uint64{4, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.computeMarket.Create(task, "Alice", available-10, 1); err != nil {
		t.Fatal(err)
	}
	tx := transferTestTx(t, alice, wallets["Bob"].Address, 11, 0)
	response := requestWalletTransfer(t, api, chainID, tx)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "unreserved") {
		t.Fatalf("reserved compute funds spent: HTTP %d: %s", response.Code, response.Body.String())
	}
}

func TestSignedWalletTransferSerializesNonce(t *testing.T) {
	api, wallets, chainID := walletTransferTestAPI(t)
	alice := wallets["Alice"]
	txA := transferTestTx(t, alice, wallets["Bob"].Address, 1, 0)
	txB := transferTestTx(t, alice, wallets["Charlie"].Address, 1, 0)
	txs := []transaction.Transaction{txA, txB}
	results := make([]int, 2)
	var group sync.WaitGroup
	for i := range txs {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			response := requestWalletTransfer(t, api, chainID, txs[index])
			results[index] = response.Code
		}(i)
	}
	group.Wait()
	success, conflicts := 0, 0
	for _, code := range results {
		if code == http.StatusCreated {
			success++
		}
		if code == http.StatusConflict {
			conflicts++
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("expected one commit and one nonce conflict, got %v", results)
	}
}

func TestWalletHistoryLimits(t *testing.T) {
	api, _, _ := walletTransferTestAPI(t)
	for _, query := range []string{
		"/api/v1/transactions", "/api/v1/transactions?address=Alice&limit=0",
		"/api/v1/transactions?address=Alice&limit=101", "/api/v1/transactions?address=Alice&limit=nope",
	} {
		response := getWalletTransferHistory(api, query)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid query %s accepted: %d", query, response.Code)
		}
	}
}
