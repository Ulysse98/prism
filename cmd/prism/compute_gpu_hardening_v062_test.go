package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"prism/internal/compute"
	"prism/internal/storage"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func freshGPUWorkerV062(t *testing.T) *wallet.Wallet {
	t.Helper()
	value, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestGPUOutputBoundsV062(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("shell fixtures require Linux")
	}
	task := gpuTaskForTestV062(t)
	for _, tc := range []struct {
		name   string
		script string
		want   string
	}{
		{"oversized_stdout", "#!/bin/sh\nhead -c 262144 /dev/zero\n", "stdout exceeds limit"},
		{"oversized_stderr", "#!/bin/sh\nhead -c 262144 /dev/zero >&2\n", "stderr exceeds limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fake-gpu")
			if err := os.WriteFile(path, []byte(tc.script), 0700); err != nil {
				t.Fatal(err)
			}
			worker := freshGPUWorkerV062(t)
			_, err := executeComputeGPUV062(task, "job", "chain", "genesis", worker, path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestGPUClaimedJobFailureAndRecoveryV062(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux worker")
	}
	gpuBinary := os.Getenv("PRISM_GPU_TASK_BIN")
	if gpuBinary == "" {
		t.Skip("set PRISM_GPU_TASK_BIN to test real CUDA recovery")
	}
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := wallets["Alice"], wallets["Bob"]
	if alice == nil || bob == nil {
		t.Fatal("missing test wallets")
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
	task := gpuTaskForTestV062(t)
	job, err := market.Create(task, "Alice", 1, 62004)
	if err != nil {
		t.Fatal(err)
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
	beforeSupply, err := before.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	beforeAlice, err := before.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	beforeBob, err := before.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}

	// Valid task, but a deliberately failing subprocess.
	// Claim may be persisted; settlement must not occur.
	broken := filepath.Join(t.TempDir(), "fake-failing-gpu")
	if err := os.WriteFile(broken, []byte("#!/bin/sh\necho simulated_cuda_failure >&2\nexit 17\n"), 0700); err != nil {
		t.Fatal(err)
	}
	args := func(binary string) []string {
		return []string{
			"-api", server.URL + "/api/v1", "-data", dataDir,
			"-compute-backend", "gpu", "-gpu-bin", binary,
			"Bob", job.ID,
		}
	}
	failed := runComputeWorkerCommand(args(broken))
	if !failed.Processed || failed.Completed {
		t.Fatalf("GPU failure unexpectedly completed: %+v", failed)
	}
	claimed, err := market.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != compute.JobStatusClaimed || claimed.Worker != bob.Address || claimed.ProofID != "" {
		t.Fatalf("failed GPU job not safely CLAIMED: %+v", claimed)
	}
	afterFailure, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterFailure.Blocks) != len(before.Blocks) {
		t.Fatal("GPU failure created a block")
	}
	afterSupply, err := afterFailure.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	afterAlice, err := afterFailure.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	afterBob, err := afterFailure.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}
	if afterSupply != beforeSupply || afterAlice != beforeAlice || afterBob != beforeBob {
		t.Fatal("GPU failure changed supply or balances")
	}
	t.Log("FAIL-CLOSED: GPU error left job CLAIMED; no block and no payout")

	// Resume the same CLAIMED job with the real CUDA executable.
	recovered := runComputeWorkerCommand(args(gpuBinary))
	if !recovered.Completed || recovered.BountyReward != job.Reward {
		t.Fatalf("GPU retry failed: %+v", recovered)
	}
	settled, err := market.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settled.Status != compute.JobStatusVerified || settled.Worker != bob.Address || settled.ProofID == "" {
		t.Fatalf("retry not VERIFIED: %+v", settled)
	}
	afterRecovery, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRecovery.Blocks) != len(before.Blocks)+1 {
		t.Fatal("GPU retry did not add exactly one block")
	}
	proof := afterRecovery.Blocks[len(afterRecovery.Blocks)-1].UsefulWork
	if len(proof) != 1 || proof[0].ID != settled.ProofID {
		t.Fatal("GPU recovered proof missing on-chain")
	}
	if err := usefulwork.VerifyProof(proof[0]); err != nil {
		t.Fatal(err)
	}
	t.Log("RECOVERY: claimed job completed via real CUDA and settled once")
}
