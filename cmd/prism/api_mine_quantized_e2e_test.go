package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"prism/internal/usefulwork"
)

// TestQuantizedMLMineAPIEndToEnd runs the actual mine-api command against
// loopback HTTP handlers and fresh, temporary node/wallet state. It does not
// use the user's data directory or require a separately running Prism node.
func TestQuantizedMLMineAPIEndToEnd(t *testing.T) {
	api, _ := newMineSubmitHTTPFixture(t, usefulwork.TaskTypeMLInferenceQuantized)
	chain, _, wallets, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	worker := wallets["Alice"]
	if worker == nil || len(chain.Blocks) == 0 {
		t.Fatal("test fixture requires Alice and a non-empty chain")
	}

	sourceHeight := chain.Blocks[len(chain.Blocks)-1].Height
	option, err := mineTaskForHeightAndType(
		sourceHeight, usefulwork.TaskTypeMLInferenceQuantized,
	)
	if err != nil {
		t.Fatal(err)
	}
	workUnits, err := usefulwork.WorkUnits(option.Task)
	if err != nil {
		t.Fatal(err)
	}
	if workUnits != 27 {
		t.Fatalf("expected 27 work units for the catalog fixture, got %d", workUnits)
	}
	emissionBefore, err := chain.GetEmissionState()
	if err != nil {
		t.Fatal(err)
	}
	wantReward, err := mineRewardForWork(
		sourceHeight+1, workUnits, emissionBefore.UsefulWorkEmission,
	)
	if err != nil {
		t.Fatal(err)
	}
	if wantReward == 0 {
		t.Fatal("fresh fixture must have an available PoUW reward")
	}

	// Capture the command's actual wire requests/responses. Channels avoid
	// sharing mutable observations between the test and HTTP handler goroutines.
	type exchange struct {
		statusCode int
		request    []byte
		response   []byte
	}
	starts := make(chan exchange, 2)
	submits := make(chan exchange, 2)
	observe := func(handler http.HandlerFunc, events chan<- exchange) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "test could not read request", http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			recorder := httptest.NewRecorder()
			handler(recorder, r)
			events <- exchange{
				statusCode: recorder.Code,
				request:    body,
				response:   append([]byte(nil), recorder.Body.Bytes()...),
			}
			for name, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", api.handleStatus)
	mux.HandleFunc("/api/v1/mine/start", observe(api.handleMineStart, starts))
	mux.HandleFunc("/api/v1/mine/submit", observe(api.handleMineSubmit, submits))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 10 * time.Second

	readStatus := func() apiStatusResponse {
		t.Helper()
		response, err := client.Get(server.URL + "/api/v1/status")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status endpoint returned HTTP %d", response.StatusCode)
		}
		var status apiStatusResponse
		if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
			t.Fatal(err)
		}
		if !status.ChainValid {
			t.Fatal("persisted test chain is invalid")
		}
		return status
	}
	takeExchange := func(events <-chan exchange, endpoint string) exchange {
		t.Helper()
		select {
		case event := <-events:
			if event.statusCode != http.StatusOK {
				t.Fatalf("%s returned HTTP %d: %s", endpoint, event.statusCode, event.response)
			}
			return event
		default:
			t.Fatalf("mine-api did not reach %s; inspect its output above", endpoint)
			return exchange{}
		}
	}

	before := readStatus()
	if before.Height != sourceHeight {
		t.Fatalf("persisted fixture height: want %d, got %d", sourceHeight, before.Height)
	}

	// This is the production CLI command function, including wallet loading,
	// task reconstruction, execution, signing and both HTTP POST requests.
	runMineAPICommand([]string{
		"-api", server.URL + "/api/v1",
		"-data", api.dataPath,
		"-task", usefulwork.TaskTypeMLInferenceQuantized,
		"Alice",
	})

	start := takeExchange(starts, "/mine/start")
	var started apiMineStartResponse
	if err := json.Unmarshal(start.response, &started); err != nil {
		t.Fatal(err)
	}
	job := started.Job
	if !reflect.DeepEqual(job.usefulWorkTask(), option.Task) {
		t.Fatal("/mine/start lost or changed task fields, including signed inputs/weights/biases")
	}
	if job.SourceChainHeight != sourceHeight || job.WorkerAddress != worker.Address {
		t.Fatalf("unexpected job source height or worker address: %+v", job)
	}
	if job.Reward != wantReward || job.Status != "READY" {
		t.Fatalf("unexpected job reward/status: reward=%d status=%s", job.Reward, job.Status)
	}

	submit := takeExchange(submits, "/mine/submit")
	var payload apiMineSubmitRequest
	if err := json.Unmarshal(submit.request, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.JobID != job.ID || payload.SourceChainHeight != sourceHeight {
		t.Fatal("client submitted a different job ID or source height")
	}
	if payload.WorkerAddress != worker.Address {
		t.Fatal("client submitted proof for the wrong wallet")
	}
	proof := usefulwork.Proof{
		ID:           payload.ProofID,
		Task:         option.Task,
		Worker:       payload.WorkerAddress,
		PublicKey:    payload.PublicKey,
		Result:       payload.Result,
		ResultValues: payload.ResultValues,
		OutputHash:   payload.OutputHash,
		Score:        payload.Score,
		Signature:    payload.Signature,
	}
	if err := usefulwork.VerifyProof(proof); err != nil {
		t.Fatalf("client's wire proof failed verification: %v", err)
	}
	// These predictions follow from the fixed catalog weights and are valid
	// for every permitted base value (11..107), not from a second model run.
	if !reflect.DeepEqual(proof.ResultValues, []uint64{0, 1, 0}) {
		t.Fatalf("catalog predictions: want [0 1 0], got %v", proof.ResultValues)
	}

	var receipt apiMineSubmitResponse
	if err := json.Unmarshal(submit.response, &receipt); err != nil {
		t.Fatal(err)
	}
	if !receipt.Verified || !receipt.Proof.Verified || receipt.Reward != wantReward {
		t.Fatalf("unexpected mining receipt: %+v", receipt)
	}
	if receipt.Proof.ProofID != proof.ID || receipt.Proof.TaskID != job.ID ||
		receipt.Proof.Task != usefulwork.TaskTypeMLInferenceQuantized ||
		receipt.Proof.WorkerAddress != worker.Address || receipt.Proof.Score != workUnits ||
		receipt.Proof.Reward != wantReward || receipt.Proof.Result != 0 ||
		receipt.Proof.OutputHash != proof.OutputHash ||
		!reflect.DeepEqual(receipt.Proof.ResultValues, proof.ResultValues) {
		t.Fatal("mining receipt does not describe the submitted quantized proof")
	}

	after := readStatus()
	if after.Height != before.Height+1 || after.Blocks != before.Blocks+1 {
		t.Fatalf("expected one new block: before=%+v after=%+v", before, after)
	}
	if receipt.Block != after.Height || receipt.Proof.Block != after.Height ||
		receipt.Proof.BlockHash != after.LastHash || receipt.TotalSupply != after.TotalSupply {
		t.Fatal("HTTP receipt does not match the persisted chain")
	}
	if after.ChainID != before.ChainID || after.GenesisHash != before.GenesisHash {
		t.Fatal("mining changed the chain identity")
	}

	// A fresh apiServer instance reloads the saved chain. Check PoUW emission
	// specifically: total supply may also include the block proposer's reward.
	reloaded := &apiServer{dataPath: api.dataPath}
	stored, _, _, err := reloaded.loadState()
	if err != nil {
		t.Fatal(err)
	}
	emissionAfter, err := stored.GetEmissionState()
	if err != nil {
		t.Fatal(err)
	}
	if emissionAfter.UsefulWorkEmission < emissionBefore.UsefulWorkEmission ||
		emissionAfter.UsefulWorkEmission-emissionBefore.UsefulWorkEmission != wantReward {
		t.Fatalf("persisted PoUW emission delta must equal %d", wantReward)
	}

	// Replay exactly the client's signed HTTP payload, now at a stale height.
	request, err := http.NewRequest(
		http.MethodPost, server.URL+"/api/v1/mine/submit", bytes.NewReader(submit.request),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	replay, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	replayBody, readErr := io.ReadAll(replay.Body)
	_ = replay.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if replay.StatusCode != http.StatusConflict ||
		!strings.Contains(string(replayBody), "stale PoUW job") {
		t.Fatalf("expected stale-job HTTP 409, got %d: %s", replay.StatusCode, replayBody)
	}
	afterReplay := readStatus()
	if afterReplay.Height != after.Height || afterReplay.Blocks != after.Blocks ||
		afterReplay.LastHash != after.LastHash || afterReplay.TotalSupply != after.TotalSupply {
		t.Fatal("rejected replay changed chain state or supply")
	}
	storedAfterReplay, _, _, err := reloaded.loadState()
	if err != nil {
		t.Fatal(err)
	}
	emissionAfterReplay, err := storedAfterReplay.GetEmissionState()
	if err != nil {
		t.Fatal(err)
	}
	if emissionAfterReplay.UsefulWorkEmission != emissionAfter.UsefulWorkEmission {
		t.Fatal("rejected replay minted an additional PoUW reward")
	}

	t.Logf("HTTP mine-api E2E: height %d -> %d; predictions [0 1 0]; score %d; PoUW reward %d; persisted chain valid; replay rejected (409)",
		before.Height, after.Height, workUnits, wantReward)
}
