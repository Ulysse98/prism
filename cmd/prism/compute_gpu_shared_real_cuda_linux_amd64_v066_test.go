//go:build linux && amd64

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// Lab only. A cooperating Prism Cloud CLI may hold the same private lock.
// When busy, Core must fail without returning a proof. Once released, the
// Core sealed launcher executes real CUDA and the independent verifier
// recomputes the result on CPU. No blockchain, payout, or hardware attestation.
func TestGPUSharedLockRealIndependentV066(t *testing.T) {
	if os.Getenv("PRISM_V066_RUN_REAL_CUDA") != "1" {
		t.Skip("explicit PRISM_V066_RUN_REAL_CUDA=1 required")
	}
	binary := os.Getenv("PRISM_V066_REAL_CUDA_BIN")
	approved := os.Getenv("PRISM_V066_APPROVED_SHA256")
	lockDir := os.Getenv("PRISM_V066_E2D_LOCK_DIR")
	if !filepath.IsAbs(binary) || !filepath.IsAbs(lockDir) || len(approved) != 64 {
		t.Fatal("absolute worker/lock paths and an independently chosen SHA-256 are required")
	}
	task, err := usefulwork.NewMLInferenceQuantizedTask(
		3, 3, 3,
		[]int64{3, -1, 2, -2, 4, 1, 1, 0, -3},
		[]int64{2, -1, 0, -1, 2, 0, 0, -1, 2},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	reference, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}

	const job = "v066-isolated-e2d-shared-lock-real-cuda"
	const chain = "prism-v066-e2d-offline"
	genesis := strings.Repeat("a", 64)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	bundle, err := executeComputeGPUApprovedWithSharedLockV066(
		ctx, task, job, chain, genesis, worker, binary, approved, lockDir,
	)
	if os.Getenv("PRISM_V066_E2D_EXPECT_BUSY") == "1" {
		if !errors.Is(err, errSharedGPUBusyV066) || bundle.Proof.ID != "" {
			t.Fatalf("Cloud-owned lock did not fail closed: err=%v proof=%q", err, bundle.Proof.ID)
		}
		t.Log("PASS Cloud-owned lock refused Core without any signed proof or CUDA dispatch")
		return
	}
	if err != nil {
		t.Fatalf("real sealed CUDA under shared lock failed: %v", err)
	}
	if !reflect.DeepEqual(bundle.Proof.ResultValues, reference) {
		t.Fatalf("CPU/GPU parity failed: GPU=%v CPU=%v", bundle.Proof.ResultValues, reference)
	}
	if bundle.Proof.ProofVersion != usefulwork.ComputeProofVersion {
		t.Fatal("expected context-bound signed PoUW v2")
	}
	report, err := usefulwork.SignComputeVerificationV064(bundle.Proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(
		bundle.Contract, bundle.Proof, report, approved, verifier.Address,
	); err != nil {
		t.Fatalf("independent CPU verification rejected real CUDA proof: %v", err)
	}
	t.Logf("PASS locked sealed real CUDA + independent verifier: proof=%s report=%s predictions=%v", bundle.Proof.ID, report.ID, reference)

	t.Run("tampered-report-rejected", func(t *testing.T) {
		altered := report
		altered.ProofID = strings.Repeat("f", 64)
		if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(
			bundle.Contract, bundle.Proof, altered, approved, verifier.Address,
		); err == nil {
			t.Fatal("tampered verifier report was accepted")
		}
	})
	t.Run("unapproved-verifier-rejected", func(t *testing.T) {
		if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(
			bundle.Contract, bundle.Proof, report, approved, worker.Address,
		); err == nil {
			t.Fatal("unapproved verifier was accepted")
		}
	})
}
