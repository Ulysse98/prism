package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prism/internal/storage"
)

func TestWalletAPIReturnsBalanceSnapshot(
	t *testing.T,
) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}

	alice := wallets["Alice"]
	if alice == nil {
		t.Fatal("Alice wallet missing")
	}

	dataPath := t.TempDir()

	if err := storage.Save(
		dataPath,
		chain,
		pos,
		wallets,
	); err != nil {
		t.Fatal(err)
	}

	api := &apiServer{
		dataPath: dataPath,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/wallets/"+alice.Address,
		nil,
	)

	response := httptest.NewRecorder()

	api.handleWalletByAddress(
		response,
		request,
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	var decoded apiWalletResponse

	if err := json.NewDecoder(
		response.Body,
	).Decode(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Address != alice.Address {
		t.Fatalf(
			"address mismatch: got %s want %s",
			decoded.Address,
			alice.Address,
		)
	}

	total, err := chain.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.TotalBalance != total {
		t.Fatalf(
			"total balance mismatch: got %d want %d",
			decoded.TotalBalance,
			total,
		)
	}

	available, err := chain.AvailableBalanceOf(
		alice.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.AvailableBalance != available {
		t.Fatalf(
			"available balance mismatch: got %d want %d",
			decoded.AvailableBalance,
			available,
		)
	}

	if decoded.LockedStake != chain.LockedStakeOf(alice.Address) {
		t.Fatalf(
			"locked stake mismatch: got %d want %d",
			decoded.LockedStake,
			chain.LockedStakeOf(alice.Address),
		)
	}
}
