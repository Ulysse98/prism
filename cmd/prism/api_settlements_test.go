package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prism/internal/crosschain"
)

func apiSettlementHex32(
	pair string,
) string {
	return "0x" +
		strings.Repeat(
			pair,
			32,
		)
}

func newSettlementTestAPI(
	t *testing.T,
) *apiServer {
	t.Helper()

	store, err :=
		crosschain.NewSettlementStore(
			t.TempDir(),
		)
	if err != nil {
		t.Fatal(err)
	}

	return &apiServer{
		settlements: store,
	}
}

func TestSettlementAPIStoresAndReturnsArbitrumConfirmation(
	t *testing.T,
) {
	api :=
		newSettlementTestAPI(t)

	payload :=
		crosschain.Settlement{
			RegistryID: apiSettlementHex32("ab"),
			Chain: crosschain.
				SettlementChainArbitrum,
			Status: crosschain.
				SettlementStatusConfirmed,
			TxHash: apiSettlementHex32("cd"),
			RegistryAddress: "0x" +
				strings.Repeat(
					"12",
					20,
				),
			BlockNumber: 311964958,
		}

	body, err :=
		json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/settlements",
			bytes.NewReader(body),
		)

	request.RemoteAddr =
		"127.0.0.1:12345"

	response :=
		httptest.NewRecorder()

	api.handleSettlements(
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

	getRequest :=
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/settlements/"+
				payload.RegistryID,
			nil,
		)

	getResponse :=
		httptest.NewRecorder()

	api.handleSettlementByRegistry(
		getResponse,
		getRequest,
	)

	if getResponse.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			getResponse.Code,
			getResponse.Body.String(),
		)
	}

	var result struct {
		RegistryID string `json:"registryId"`

		Settlements []crosschain.Settlement `json:"settlements"`
	}

	if err :=
		json.NewDecoder(
			getResponse.Body,
		).Decode(&result); err != nil {

		t.Fatal(err)
	}

	if result.RegistryID !=
		payload.RegistryID {

		t.Fatalf(
			"registry ID mismatch: %s",
			result.RegistryID,
		)
	}

	if len(result.Settlements) != 1 {
		t.Fatalf(
			"expected 1 settlement, got %d",
			len(result.Settlements),
		)
	}

	if result.Settlements[0].TxHash !=
		payload.TxHash {

		t.Fatal(
			"settlement transaction hash mismatch",
		)
	}
}

func TestSettlementAPIRejectsRemoteWriter(
	t *testing.T,
) {
	api :=
		newSettlementTestAPI(t)

	payload :=
		crosschain.Settlement{
			RegistryID: apiSettlementHex32("ab"),
			Chain: crosschain.
				SettlementChainArbitrum,
			Status: crosschain.
				SettlementStatusConfirmed,
			TxHash: apiSettlementHex32("cd"),
			RegistryAddress: "0x" +
				strings.Repeat(
					"12",
					20,
				),
			BlockNumber: 1,
		}

	body, err :=
		json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/settlements",
			bytes.NewReader(body),
		)

	request.RemoteAddr =
		"203.0.113.42:12345"

	response :=
		httptest.NewRecorder()

	api.handleSettlements(
		response,
		request,
	)

	if response.Code !=
		http.StatusForbidden {

		t.Fatalf(
			"expected HTTP 403, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}
}
