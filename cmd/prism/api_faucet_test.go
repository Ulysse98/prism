package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prism/internal/storage"
)

func TestFaucetDisabledByDefault(
	t *testing.T,
) {
	api := &apiServer{}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/faucet",
		bytes.NewBufferString(
			`{"address":"prism_0000000000000000000000000000000000000000"}`,
		),
	)

	response := httptest.NewRecorder()

	api.handleFaucet(
		response,
		request,
	)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			http.StatusForbidden,
			response.Code,
			response.Body.String(),
		)
	}
}

func TestFaucetFundsAddressOnce(
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

	bob := wallets["Bob"]
	if bob == nil {
		t.Fatal("Bob wallet missing")
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
		dataPath:      dataPath,
		faucetEnabled: true,
		faucetWallet:  "Alice",
		faucetAmount:  100,
	}

	body := []byte(
		`{"address":"` +
			bob.Address +
			`"}`,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/faucet",
		bytes.NewReader(body),
	)

	response := httptest.NewRecorder()

	api.handleFaucet(
		response,
		request,
	)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var decoded apiFaucetResponse

	if err := json.NewDecoder(
		response.Body,
	).Decode(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Address != bob.Address {
		t.Fatalf(
			"recipient mismatch: got %s want %s",
			decoded.Address,
			bob.Address,
		)
	}

	if decoded.Amount != 100 {
		t.Fatalf(
			"amount mismatch: got %d want 100",
			decoded.Amount,
		)
	}

	if decoded.TxID == "" {
		t.Fatal(
			"expected faucet transaction ID",
		)
	}

	updatedChain, _, _, err :=
		storage.Load(dataPath)
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, block := range updatedChain.Blocks {
		for _, tx := range block.Transactions {
			if tx.ID != decoded.TxID {
				continue
			}

			if tx.From != alice.Address {
				t.Fatalf(
					"unexpected faucet sender: %s",
					tx.From,
				)
			}

			if tx.To != bob.Address {
				t.Fatalf(
					"unexpected faucet recipient: %s",
					tx.To,
				)
			}

			if tx.Amount != 100 {
				t.Fatalf(
					"unexpected faucet amount: %d",
					tx.Amount,
				)
			}

			found = true
		}
	}

	if !found {
		t.Fatal(
			"faucet transaction not found on-chain",
		)
	}

	// A second claim from the same address must fail.
	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/faucet",
		bytes.NewReader(body),
	)

	secondResponse := httptest.NewRecorder()

	api.handleFaucet(
		secondResponse,
		secondRequest,
	)

	if secondResponse.Code !=
		http.StatusConflict {

		t.Fatalf(
			"expected HTTP %d for duplicate claim, got %d: %s",
			http.StatusConflict,
			secondResponse.Code,
			secondResponse.Body.String(),
		)
	}
}

func TestFaucetRejectsGET(
	t *testing.T,
) {
	api := &apiServer{}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/faucet",
		nil,
	)

	response := httptest.NewRecorder()

	api.handleFaucet(
		response,
		request,
	)

	if response.Code !=
		http.StatusMethodNotAllowed {

		t.Fatalf(
			"expected HTTP %d, got %d",
			http.StatusMethodNotAllowed,
			response.Code,
		)
	}
}

func TestFaucetFundsExternalAddress(
	t *testing.T,
) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
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
		dataPath:      dataPath,
		faucetEnabled: true,
		faucetWallet:  "Alice",
		faucetAmount:  100,
	}

	const address = "prism_5252525252525252525252525252525252525252"

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/faucet",
		bytes.NewBufferString(
			`{"address":"`+address+`"}`,
		),
	)

	response := httptest.NewRecorder()

	api.handleFaucet(
		response,
		request,
	)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	updatedChain, _, _, err :=
		storage.Load(dataPath)
	if err != nil {
		t.Fatal(err)
	}

	balance, err :=
		updatedChain.BalanceOf(address)
	if err != nil {
		t.Fatal(err)
	}

	if balance != 100 {
		t.Fatalf(
			"external wallet balance mismatch: got %d want 100",
			balance,
		)
	}
}

func TestFaucetRejectsSelfFunding(
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
		dataPath:      dataPath,
		faucetEnabled: true,
		faucetWallet:  "Alice",
		faucetAmount:  100,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/faucet",
		bytes.NewBufferString(
			`{"address":"`+alice.Address+`"}`,
		),
	)

	response := httptest.NewRecorder()

	api.handleFaucet(
		response,
		request,
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}
}
