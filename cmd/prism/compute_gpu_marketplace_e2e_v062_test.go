package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

// TestGPUComputeMarketplaceEndToEndV062 uses ONLY t.TempDir() chain,
// wallets and marketplace. It reaches the real HTTP claim/complete
// handlers and runs the real CUDA executable via the production CLI.
func TestGPUComputeMarketplaceEndToEndV062(t *testing.T) {
	gpuBin := os.Getenv("PRISM_GPU_TASK_BIN")
	if gpuBin == "" {
		t.Skip("set PRISM_GPU_TASK_BIN for real CUDA E2E")
	}

	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	alice := wallets["Alice"]
	bob := wallets["Bob"]
	if alice == nil || bob == nil {
		t.Fatal("missing temporary test wallets")
	}

	dataDir := t.TempDir()
	if err := storage.Save(dataDir, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}

	market, err := compute.NewPersistentMarketplace(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	registerComputeMarketplaceCleanup(t, market)

	task, err := usefulwork.NewMLInferenceQuantizedTask(
		3, 3, 3,
		[]int64{2, -3, 1, -1, 4, 2, 0, -2, 5},
		[]int64{2, 1, -1, -1, -2, 2, 1, 1, 1},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(task, "Alice", 1, 62001)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != compute.JobStatusOpen {
		t.Fatal("job is not OPEN")
	}

	api := &apiServer{dataPath: dataDir, computeMarket: market}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", api.handleStatus)
	mux.HandleFunc("/api/v1/compute/jobs/", api.handleComputeJobAction)
	server := httptest.NewServer(mux)
	defer server.Close()

	before, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	beforeBlocks := len(before.Blocks)
	if beforeBlocks == 0 {
		t.Fatal("test chain has no genesis block")
	}
	chainID := p2p.MakeChainID(before.Blocks[0].Hash)
	genesis := before.Blocks[0].Hash

	// Same entrypoint a user invokes, including HTTP discovery,
	// signed claim, CUDA execution, V2 proof, API settlement.
	result := runComputeWorkerCommand([]string{
		"-api", server.URL + "/api/v1",
		"-data", dataDir,
		"-compute-backend", "gpu",
		"-gpu-bin", gpuBin,
		"Bob", job.ID,
	})
	if !result.Processed || !result.Completed || result.JobID != job.ID || result.BountyReward != job.Reward {
		t.Fatalf("real CLI GPU job not completed: %+v", result)
	}

	verifiedJob, err := market.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if verifiedJob.Status != compute.JobStatusVerified || verifiedJob.Worker != bob.Address {
		t.Fatalf("invalid marketplace state: %+v", verifiedJob)
	}

	after, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Blocks) != beforeBlocks+1 {
		t.Fatalf("expected exactly one added block, got %d -> %d", beforeBlocks, len(after.Blocks))
	}
	block := after.Blocks[len(after.Blocks)-1]
	if len(block.UsefulWork) != 1 {
		t.Fatalf("expected one on-chain proof, got %d", len(block.UsefulWork))
	}
	proof := block.UsefulWork[0]
	if proof.ProofVersion != usefulwork.ComputeProofVersion ||
		proof.Worker != bob.Address || proof.Task.ID != job.Task.ID ||
		proof.ID != verifiedJob.ProofID ||
		!reflect.DeepEqual(proof.ResultValues, []uint64{1, 2, 1}) {
		t.Fatalf("on-chain GPU proof mismatch: %+v", proof)
	}
	if err := usefulwork.VerifyProof(proof); err != nil {
		t.Fatal(err)
	}
	if err := usefulwork.VerifyComputeProofContext(proof, job.ID, chainID, genesis); err != nil {
		t.Fatal(err)
	}

	// Settlement must contain one requester -> worker bounty transfer.
	transfers := 0
	var settlementTxID string
	for _, tx := range block.Transactions {
		if tx.From == alice.Address && tx.To == bob.Address && tx.Amount == job.Reward {
			transfers++
			settlementTxID = tx.ID
		}
	}
	if transfers != 1 || settlementTxID == "" {
		t.Fatalf("expected exactly one bounty transfer, got %d", transfers)
	}

	supplyBeforeRetry, err := after.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	aliceBeforeRetry, err := after.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	bobBeforeRetry, err := after.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}

	// Retry the exact completion proof over HTTP. The node must recover
	// the existing settlement without creating a new block or paying twice.
	encoded, err := json.Marshal(apiComputeCompleteJobRequest{Proof: proof})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Post(
		server.URL+"/api/v1/compute/jobs/"+job.ID+"/complete",
		"application/json", bytes.NewReader(encoded),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("settlement retry HTTP status: %d", response.StatusCode)
	}
	var replay apiComputeWorkerCompleteResponse
	if err := json.NewDecoder(response.Body).Decode(&replay); err != nil {
		t.Fatal(err)
	}
	if !replay.Verified || !replay.Settled || !replay.Recovered ||
		replay.SettlementTxID != settlementTxID || replay.Job.ProofID != proof.ID {
		t.Fatalf("settlement retry not idempotent: %+v", replay)
	}
	afterRetry, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRetry.Blocks) != len(after.Blocks) ||
		afterRetry.Blocks[len(afterRetry.Blocks)-1].Hash != block.Hash {
		t.Fatal("replay changed chain height or block hash")
	}
	supplyAfterRetry, err := afterRetry.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	aliceAfterRetry, err := afterRetry.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	bobAfterRetry, err := afterRetry.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}
	if supplyAfterRetry != supplyBeforeRetry ||
		aliceAfterRetry != aliceBeforeRetry || bobAfterRetry != bobBeforeRetry {
		t.Fatal("replay changed supply or wallet balances")
	}
	t.Logf("REAL CUDA CLI -> CLAIM -> VERIFIED -> ON-CHAIN BOUNTY PASS: proof=%s tx=%s", proof.ID, settlementTxID)
	t.Log("RETRY: same block, transaction, supply and wallet balances")

	// A GPU-incompatible task must be rejected before claim.
	unsupportedTask, err := usefulwork.NewMLInferenceQuantizedTask(
		2, 2, 2,
		[]int64{1, 2, 3, 4},
		[]int64{1, 0, 0, 1},
		[]int64{0, 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	unsupported, err := market.Create(unsupportedTask, "Alice", 1, 62002)
	if err != nil {
		t.Fatal(err)
	}
	rejected := runComputeWorkerCommand([]string{
		"-api", server.URL + "/api/v1",
		"-data", dataDir,
		"-compute-backend", "gpu",
		"-gpu-bin", gpuBin,
		"Bob", unsupported.ID,
	})
	if rejected.Completed {
		t.Fatal("unsupported GPU job completed")
	}
	pending, err := market.Get(unsupported.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != compute.JobStatusOpen || pending.Worker != "" {
		t.Fatalf("unsupported job was claimed: %+v", pending)
	}
	last, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Blocks) != len(afterRetry.Blocks) {
		t.Fatal("rejected GPU task modified the chain")
	}
	t.Log("UNSUPPORTED 2x2 GPU TASK: rejected before claim")
}
