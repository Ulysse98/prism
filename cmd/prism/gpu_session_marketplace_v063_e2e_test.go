package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// This file is a TEMPORARY, offline-only integration lab. Production CLI,
// consensus and the marketplace's stored data are never modified on disk.
// All chain, wallet and API state is created solely inside t.TempDir().
func v063MarketplaceLabGPU(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("persistent CUDA session lab requires Linux/WSL2")
	}
	bin := os.Getenv("PRISM_V063_E2E_GPU_BIN")
	if bin == "" {
		t.Skip("set PRISM_V063_E2E_GPU_BIN for isolated real CUDA E2E")
	}
	if err := validateGPUWorkerBinaryV062(bin); err != nil {
		t.Fatalf("GPU binary invalid: %v", err)
	}
	return bin
}

type v063MarketLab struct {
	api     *apiServer
	market  *compute.Marketplace
	server  *httptest.Server
	dataDir string
	alice   *wallet.Wallet
	bob     *wallet.Wallet
	chainID string
	genesis string
	baseURL string
}

func newV063MarketLab(t *testing.T) *v063MarketLab {
	t.Helper()
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := wallets["Alice"], wallets["Bob"]
	if alice == nil || bob == nil {
		t.Fatal("temporary node wallets missing")
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
	api := &apiServer{dataPath: dataDir, computeMarket: market}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", api.handleStatus)
	mux.HandleFunc("/api/v1/compute/jobs", api.handleComputeJobs)
	mux.HandleFunc("/api/v1/compute/jobs/", api.handleComputeJobAction)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	before, _, _, err := api.loadState()
	if err != nil || len(before.Blocks) == 0 {
		t.Fatalf("temporary chain unavailable: %v", err)
	}
	genesis := before.Blocks[0].Hash
	return &v063MarketLab{
		api: api, market: market, server: server, dataDir: dataDir,
		alice: alice, bob: bob, chainID: p2p.MakeChainID(genesis),
		genesis: genesis, baseURL: server.URL + "/api/v1",
	}
}

func v063MarketTask(t *testing.T, delta int64) usefulwork.Task {
	t.Helper()
	inputs := []int64{2 + delta, -3, 1, -1, 4, 2, 0, -2, 5}
	task, err := usefulwork.NewMLInferenceQuantizedTask(3, 3, 3, inputs,
		[]int64{2, 1, -1, -1, -2, 2, 1, 1, 1}, []int64{1, -1, 0})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func (l *v063MarketLab) createAndClaim(t *testing.T, task usefulwork.Task, nonce uint64) compute.Job {
	t.Helper()
	job, err := l.market.Create(task, "Alice", 1, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != compute.JobStatusOpen {
		t.Fatalf("job not OPEN: %s", job.Status)
	}
	claim, err := compute.SignClaimAuthorization(job.ID, l.chainID, l.genesis, l.bob)
	if err != nil {
		t.Fatal(err)
	}
	var response apiComputeJobResponse
	if err := mineAPIPost(l.server.Client(), l.baseURL+"/compute/jobs/"+job.ID+"/claim", claim, &response); err != nil {
		t.Fatalf("signed claim rejected: %v", err)
	}
	if response.Job.ID != job.ID || response.Job.Worker != l.bob.Address ||
		response.Job.Status != compute.JobStatusClaimed {
		t.Fatalf("wrong claim response: %+v", response.Job)
	}
	return response.Job
}

func (l *v063MarketLab) chainSnapshot(t *testing.T) (height int, supply, aliceBalance, bobBalance uint64) {
	t.Helper()
	chain, _, _, err := l.api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	supply, err = chain.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	aliceBalance, err = chain.BalanceOf(l.alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	bobBalance, err = chain.BalanceOf(l.bob.Address)
	if err != nil {
		t.Fatal(err)
	}
	return len(chain.Blocks), supply, aliceBalance, bobBalance
}

func (l *v063MarketLab) finishFromGPU(t *testing.T, job compute.Job, values []uint64) (usefulwork.Proof, apiComputeWorkerCompleteResponse) {
	t.Helper()
	proof, err := usefulwork.BuildComputeMLProofV2FromResult(job.Task, job.ID, l.chainID, l.genesis, l.bob, values)
	if err != nil {
		t.Fatalf("cannot build V2 proof: %v", err)
	}
	if err := usefulwork.VerifyProof(proof); err != nil {
		t.Fatalf("signed V2 proof invalid: %v", err)
	}
	if err := usefulwork.VerifyComputeProofContext(proof, job.ID, l.chainID, l.genesis); err != nil {
		t.Fatalf("proof context invalid: %v", err)
	}
	cpu, err := usefulwork.ExecuteCompute(job.Task, job.ID, l.chainID, l.genesis, l.bob)
	if err != nil || !reflect.DeepEqual(cpu, proof) {
		t.Fatalf("GPU proof differs from CPU: %v", err)
	}
	var response apiComputeWorkerCompleteResponse
	if err := mineAPIPost(l.server.Client(), l.baseURL+"/compute/jobs/"+job.ID+"/complete",
		apiComputeCompleteJobRequest{Proof: proof}, &response); err != nil {
		t.Fatalf("API did not verify/settle GPU proof: %v", err)
	}
	if err := validateComputeCompletion(response, job, proof); err != nil {
		t.Fatalf("invalid completion: %v", err)
	}
	chain, _, _, err := l.api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	block := chain.Blocks[len(chain.Blocks)-1]
	if len(block.UsefulWork) != 1 || block.UsefulWork[0].ID != proof.ID {
		t.Fatalf("settlement block lacks exact signed GPU proof")
	}
	transfers := 0
	for _, tx := range block.Transactions {
		if tx.From == l.alice.Address && tx.To == l.bob.Address &&
			tx.Amount == job.Reward && tx.ID == response.SettlementTxID {
			transfers++
		}
	}
	if transfers != 1 {
		t.Fatalf("expected exactly one requester-to-worker bounty transfer, got %d", transfers)
	}
	return proof, response
}

func (l *v063MarketLab) replayCompletion(t *testing.T, job compute.Job, proof usefulwork.Proof, previous apiComputeWorkerCompleteResponse) {
	t.Helper()
	h0, s0, a0, b0 := l.chainSnapshot(t)
	encoded, err := json.Marshal(apiComputeCompleteJobRequest{Proof: proof})
	if err != nil {
		t.Fatal(err)
	}
	response, err := l.server.Client().Post(l.baseURL+"/compute/jobs/"+job.ID+"/complete", "application/json", bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("settlement replay HTTP status: %d", response.StatusCode)
	}
	var replay apiComputeWorkerCompleteResponse
	if err := json.NewDecoder(response.Body).Decode(&replay); err != nil {
		t.Fatal(err)
	}
	if !replay.Verified || !replay.Settled || !replay.Recovered ||
		replay.SettlementTxID != previous.SettlementTxID || replay.Job.ProofID != proof.ID {
		t.Fatalf("settlement replay not idempotent: %+v", replay)
	}
	h1, s1, a1, b1 := l.chainSnapshot(t)
	if h0 != h1 || s0 != s1 || a0 != a1 || b0 != b1 {
		t.Fatalf("replay modified chain height, supply or balances: %d/%d", h0, h1)
	}
}

func TestGPUV063MarketSessionTwoJobsOneProcessLab(t *testing.T) {
	bin := v063MarketplaceLabGPU(t)
	l := newV063MarketLab(t)
	s, err := startGPUV063Supervisor(bin, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for i := 0; i < 2; i++ {
		task := v063MarketTask(t, int64(i))
		job := l.createAndClaim(t, task, uint64(63001+i))
		h0, _, _, _ := l.chainSnapshot(t)
		values, err := s.Execute(context.Background(), job.Task, i)
		if err != nil {
			t.Fatalf("GPU persistent request %d failed: %v; stderr=%s", i, err, s.stderr.Detail())
		}
		proof, completion := l.finishFromGPU(t, job, values)
		h1, _, _, _ := l.chainSnapshot(t)
		if h1 != h0+1 {
			t.Fatalf("job %d: wanted exactly one settlement block, got %d -> %d", i, h0, h1)
		}
		l.replayCompletion(t, job, proof, completion)
		verifiedJob, err := l.market.Get(job.ID)
		if err != nil || verifiedJob.Status != compute.JobStatusVerified {
			t.Fatalf("job not VERIFIED: %+v (%v)", verifiedJob, err)
		}
		t.Logf("PASS: job %d GPU session -> signed claim -> V2 proof -> single settlement -> replay idempotent", i+1)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: 2 distinct marketplace jobs completed in one CUDA session; test chain only")
}

func TestGPUV063MarketSessionFailureNoPayoutAndRecoveryLab(t *testing.T) {
	bin := v063MarketplaceLabGPU(t)
	for _, tc := range []struct {
		name, fakeScript string
		timeout          time.Duration
	}{
		{"process_crash", "#!/bin/sh\nIFS= read -r request || exit 18\nprintf 'simulated_cuda_failure\\n' >&2\nexit 17\n", 2 * time.Second},
		{"process_timeout", "#!/bin/sh\nIFS= read -r request || exit 18\nsleep 30 &\nwait\n", 350 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newV063MarketLab(t)
			job := l.createAndClaim(t, v063MarketTask(t, 2), 63003)
			h0, s0, a0, b0 := l.chainSnapshot(t)
			fake := filepath.Join(t.TempDir(), "fake-cuda")
			if err := os.WriteFile(fake, []byte(tc.fakeScript), 0700); err != nil {
				t.Fatal(err)
			}
			broken, err := startGPUV063Supervisor(fake, tc.timeout)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := broken.Execute(context.Background(), job.Task, 0); err == nil || !broken.IsDead() {
				t.Fatalf("bad CUDA execution was not rejected/poisoned: %v", err)
			}
			_ = broken.Close()
			state, err := l.market.Get(job.ID)
			if err != nil || state.Status != compute.JobStatusClaimed || state.Worker != l.bob.Address {
				t.Fatalf("GPU failure must leave job CLAIMED: %+v (%v)", state, err)
			}
			h1, s1, a1, b1 := l.chainSnapshot(t)
			if h1 != h0 || s1 != s0 || a1 != a0 || b1 != b0 {
				t.Fatal("GPU failure changed blocks, supply or balances")
			}
			t.Log("PASS: GPU failure produced no proof submission, block or bounty")
			recovered, err := startGPUV063Supervisor(bin, 10*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = recovered.Close() })
			values, err := recovered.Execute(context.Background(), state.Task, 0)
			if err != nil {
				t.Fatalf("real CUDA recovery failed: %v", err)
			}
			proof, completion := l.finishFromGPU(t, state, values)
			h2, _, _, _ := l.chainSnapshot(t)
			if h2 != h0+1 {
				t.Fatal("recovered job did not settle in exactly one new block")
			}
			l.replayCompletion(t, state, proof, completion)
			if err := recovered.Close(); err != nil {
				t.Fatal(err)
			}
			t.Log("PASS: fresh CUDA session completed claimed job, V2 settlement once, replay idempotent")
		})
	}
}

func TestGPUV063MarketSessionRejectUnsupportedBeforeClaimLab(t *testing.T) {
	_ = v063MarketplaceLabGPU(t)
	l := newV063MarketLab(t)
	unsupported, err := usefulwork.NewMLInferenceQuantizedTask(2, 2, 2,
		[]int64{1, 2, 3, 4}, []int64{1, 0, 0, 1}, []int64{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	job, err := l.market.Create(unsupported, "Alice", 1, 63010)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateGPUComputeTaskV062(job.Task); err == nil {
		t.Fatal("unsupported 2x2 was accepted by GPU validation")
	}
	stored, err := l.market.Get(job.ID)
	if err != nil || stored.Status != compute.JobStatusOpen || strings.TrimSpace(stored.Worker) != "" {
		t.Fatalf("unsupported task was claimed: %+v (%v)", stored, err)
	}
	t.Log("PASS: unsupported task rejected before claim on isolated marketplace")
}
