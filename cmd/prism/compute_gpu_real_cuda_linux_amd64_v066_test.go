//go:build linux && amd64

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// This integration test requires explicit opt-in. It invokes an actual CUDA
// worker binary through the approved sealed one-shot v0.66 path. No network,
// escrow operation, blockchain mutation, or hardware attestation is involved.
func TestGPUApprovedRealCUDASmokeV066(t *testing.T) {
	if os.Getenv("PRISM_V066_RUN_REAL_CUDA") != "1" {
		t.Skip("real CUDA smoke requires PRISM_V066_RUN_REAL_CUDA=1")
	}
	bin := os.Getenv("PRISM_V066_REAL_CUDA_BIN")
	digest := os.Getenv("PRISM_V066_APPROVED_SHA256")
	if !filepath.IsAbs(bin) || len(digest) != 64 {
		t.Fatal("an absolute CUDA binary path and a pinned SHA-256 are required")
	}

	// Three actual samples, three input features, three output classes.
	// Deterministic independent CPU reference; no real network identity.
	task, err := usefulwork.NewMLInferenceQuantizedTask(
		3, 3, 3,
		[]int64{3, -1, 2, -2, 4, 1, 1, 0, -3},
		[]int64{2, -1, 0, -1, 2, 0, 0, -1, 2},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	const jobID = "v066-isolated-cuda-smoke-1d3"
	const chainID = "prism-v066-isolated-smoke"
	genesisHash := strings.Repeat("a", 64)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	bundle, err := executeComputeGPUApprovedV066(
		ctx, task, jobID, chainID, genesisHash, worker, bin, digest,
	)
	if err != nil {
		t.Fatalf("real CUDA sealed execution failed closed: %v", err)
	}
	if !reflect.DeepEqual(bundle.Proof.ResultValues, reference) {
		t.Fatalf("GPU predictions differ from CPU reference: GPU=%v CPU=%v", bundle.Proof.ResultValues, reference)
	}
	if bundle.Proof.ProofVersion != usefulwork.ComputeProofVersion {
		t.Fatal("GPU proof does not use context-bound PoUW v2")
	}
	if bundle.Contract.ArtifactHash != digest {
		t.Fatal("GPU contract does not bind the approved artifact")
	}
	if err := usefulwork.VerifyQuantizedMLComputeTaskBindingV066(bundle.Contract, bundle.Proof, digest); err != nil {
		t.Fatalf("independent contract/proof verification failed: %v", err)
	}
	t.Logf("PASS real worker / sealed launch / CPU parity / PoUW v2: task=%s proof=%s predictions=%v", task.ID, bundle.Proof.ID, reference)

	t.Run("altered-result-rejected", func(t *testing.T) {
		mutated := bundle.Proof
		mutated.ResultValues = append([]uint64(nil), bundle.Proof.ResultValues...)
		if len(mutated.ResultValues) == 0 {
			t.Fatal("expected non-empty results")
		}
		mutated.ResultValues[0] = (mutated.ResultValues[0] + 1) % task.ColsB
		if err := usefulwork.VerifyQuantizedMLComputeTaskBindingV066(bundle.Contract, mutated, digest); err == nil {
			t.Fatal("tampered real CUDA result unexpectedly accepted")
		}
	})
	t.Run("wrong-job-contract-rejected", func(t *testing.T) {
		mutated := bundle.Contract
		mutated.JobID = "other-job"
		if err := usefulwork.VerifyQuantizedMLComputeTaskBindingV066(mutated, bundle.Proof, digest); err == nil {
			t.Fatal("wrong job contract unexpectedly accepted")
		}
	})
	t.Run("unapproved-worker-hash-rejected", func(t *testing.T) {
		invalidDigest := strings.Repeat("0", 64)
		if invalidDigest == digest {
			t.Fatal("invalid digest collision with allowlist")
		}
		rejected, err := executeComputeGPUApprovedV066(
			context.Background(), task, jobID, chainID, genesisHash, worker, bin, invalidDigest,
		)
		if err == nil || rejected.Proof.ID != "" {
			t.Fatal("unapproved worker returned an apparently valid proof")
		}
	})
	t.Run("cancelled-request-rejected", func(t *testing.T) {
		cancelled, stop := context.WithCancel(context.Background())
		stop()
		rejected, err := executeComputeGPUApprovedV066(
			cancelled, task, jobID, chainID, genesisHash, worker, bin, digest,
		)
		if err == nil || rejected.Proof.ID != "" {
			t.Fatal("cancelled request returned an apparently valid proof")
		}
	})
}
