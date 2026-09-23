package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prism/internal/crosschain"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestValidateMineSourceHeightAcceptsCurrent(
	t *testing.T,
) {
	if err :=
		validateMineSourceHeight(
			44,
			44,
		); err != nil {

		t.Fatalf(
			"current PoUW height rejected: %v",
			err,
		)
	}
}

func TestValidateMineSourceHeightRejectsStale(
	t *testing.T,
) {
	err :=
		validateMineSourceHeight(
			45,
			44,
		)

	if err == nil {
		t.Fatal(
			"expected stale PoUW job rejection",
		)
	}

	if !strings.Contains(
		err.Error(),
		"stale PoUW job",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func newMineSubmitHTTPFixture(
	t *testing.T,
	taskType string,
) (
	*apiServer,
	apiMineSubmitRequest,
) {
	t.Helper()

	chain, pos, wallets, err :=
		createNode()
	if err != nil {
		t.Fatal(err)
	}

	if len(chain.Blocks) == 0 {
		t.Fatal("test chain is empty")
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	option, err :=
		mineTaskForHeightAndType(
			lastBlock.Height,
			taskType,
		)
	if err != nil {
		t.Fatal(err)
	}

	alice := wallets["Alice"]
	if alice == nil {
		t.Fatal(
			"Alice wallet missing from fixture",
		)
	}

	proof, err :=
		usefulwork.Execute(
			option.Task,
			alice,
		)
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

	api :=
		&apiServer{
			dataPath: dataPath,
		}

	payload :=
		apiMineSubmitRequest{
			JobID:             proof.Task.ID,
			SourceChainHeight: lastBlock.Height,
			WorkerAddress:     proof.Worker,
			PublicKey:         proof.PublicKey,
			Result:            proof.Result,
			ResultValues: append(
				[]uint64(nil),
				proof.ResultValues...,
			),
			OutputHash: proof.OutputHash,
			Score:      proof.Score,
			ProofID:    proof.ID,
			Signature:  proof.Signature,
		}

	return api, payload
}

func performMineSubmit(
	t *testing.T,
	api *apiServer,
	payload apiMineSubmitRequest,
) *httptest.ResponseRecorder {
	t.Helper()

	body, err :=
		json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/mine/submit",
			bytes.NewReader(body),
		)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response :=
		httptest.NewRecorder()

	api.handleMineSubmit(
		response,
		request,
	)

	return response
}

func TestHandleMineSubmitAcceptsValidConvolution(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeImageConvolution,
		)

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	chain, _, _, err :=
		api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	if lastBlock.Height !=
		payload.SourceChainHeight+1 {

		t.Fatalf(
			"expected height %d, got %d",
			payload.SourceChainHeight+1,
			lastBlock.Height,
		)
	}
}

func TestHandleMineSubmitAcceptsValidMLInference(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeMLInferenceBatch,
		)

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}
}

func TestHandleMineSubmitAcceptsValidQuantizedMLInference(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeMLInferenceQuantized,
		)

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}
}

func TestHandleMineSubmitRejectsReplayAsStale(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeImageConvolution,
		)

	first :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if first.Code != http.StatusOK {
		t.Fatalf(
			"initial proof failed: %d: %s",
			first.Code,
			first.Body.String(),
		)
	}

	second :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if second.Code !=
		http.StatusConflict {

		t.Fatalf(
			"expected HTTP 409 replay rejection, got %d: %s",
			second.Code,
			second.Body.String(),
		)
	}

	if !strings.Contains(
		second.Body.String(),
		"stale PoUW job",
	) {
		t.Fatalf(
			"unexpected replay response: %s",
			second.Body.String(),
		)
	}
}

func TestHandleMineSubmitRejectsUnknownJob(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeImageConvolution,
		)

	payload.JobID =
		"not-a-real-job"

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected HTTP 400, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"invalid PoUW job ID",
	) {
		t.Fatalf(
			"unexpected response: %s",
			response.Body.String(),
		)
	}
}

func TestHandleMineSubmitRejectsWrongScore(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeImageConvolution,
		)

	payload.Score++

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected HTTP 400, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"invalid useful work score",
	) {
		t.Fatalf(
			"unexpected response: %s",
			response.Body.String(),
		)
	}
}

func TestHandleMineSubmitRejectsTamperedVector(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeImageConvolution,
		)

	if len(payload.ResultValues) == 0 {
		t.Fatal(
			"expected vector result",
		)
	}

	payload.ResultValues[0]++

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected HTTP 400, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"invalid useful work vector result",
	) {
		t.Fatalf(
			"unexpected response: %s",
			response.Body.String(),
		)
	}
}

func TestHandleMineSubmitRejectsBadSignature(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.
				TaskTypeImageConvolution,
		)

	payload.Signature =
		strings.Repeat(
			"00",
			64,
		)

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected HTTP 400, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"invalid useful work signature",
	) {
		t.Fatalf(
			"unexpected response: %s",
			response.Body.String(),
		)
	}
}

func TestHandleMineSubmitReturnsCanonicalCrossChainReceipt(
	t *testing.T,
) {
	api, payload :=
		newMineSubmitHTTPFixture(
			t,
			usefulwork.TaskTypeSumSquares,
		)

	response :=
		performMineSubmit(
			t,
			api,
			payload,
		)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	var body struct {
		Verified          bool               `json:"verified"`
		Proof             apiWorkResponse    `json:"proof"`
		CrossChainReceipt crosschain.Receipt `json:"crossChainReceipt"`
	}

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(&body); err != nil {

		t.Fatalf(
			"cannot decode mine submit response: %v",
			err,
		)
	}

	if !body.Verified {
		t.Fatal(
			"expected verified mine response",
		)
	}

	if body.Proof.ProofID !=
		payload.ProofID {

		t.Fatalf(
			"proof ID mismatch: got %s want %s",
			body.Proof.ProofID,
			payload.ProofID,
		)
	}

	chain, _, _, err :=
		api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	if len(chain.Blocks) == 0 {
		t.Fatal(
			"chain is empty after mine submit",
		)
	}

	genesis :=
		chain.Blocks[0]

	expected, err :=
		crosschain.NewReceipt(
			payload.JobID,
			payload.ProofID,
			payload.WorkerAddress,
			p2p.MakeChainID(
				genesis.Hash,
			),
		)
	if err != nil {
		t.Fatal(err)
	}

	if body.CrossChainReceipt !=
		expected {

		t.Fatalf(
			"cross-chain receipt mismatch:\ngot:  %+v\nwant: %+v",
			body.CrossChainReceipt,
			expected,
		)
	}

	if body.CrossChainReceipt.RegistryID == "" {
		t.Fatal(
			"cross-chain registry ID is empty",
		)
	}
}
