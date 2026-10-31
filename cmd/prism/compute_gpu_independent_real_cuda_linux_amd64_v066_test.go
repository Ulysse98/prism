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

// Explicitly opt-in lab: real CUDA calculation, trusted executable hash,
// fresh worker identity, and a DIFFERENT verifier signing after CPU replay.
// No blockchain, network, settlement, or payment is involved.
func TestGPUApprovedIndependentRealCUDAV066(t *testing.T) {
	if os.Getenv("PRISM_V066_RUN_REAL_CUDA") != "1" {
		t.Skip("real independent CUDA qualification requires PRISM_V066_RUN_REAL_CUDA=1")
	}
	bin := os.Getenv("PRISM_V066_REAL_CUDA_BIN")
	digest := os.Getenv("PRISM_V066_APPROVED_SHA256")
	if !filepath.IsAbs(bin) || len(digest) != 64 {
		t.Fatal("absolute worker binary and pinned SHA-256 required")
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	bundle, err := executeComputeGPUApprovedV066(
		ctx, task, "v066-independent-real-cuda-1e1", "prism-v066-isolated-smoke",
		strings.Repeat("a", 64), worker, bin, digest,
	)
	if err != nil {
		t.Fatalf("sealed CUDA execution failed: %v", err)
	}

	cpu, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cpu, bundle.Proof.ResultValues) {
		t.Fatalf("GPU and CPU predictions differ: GPU=%v CPU=%v", bundle.Proof.ResultValues, cpu)
	}
	report, err := usefulwork.SignComputeVerificationV064(bundle.Proof, verifier)
	if err != nil {
		t.Fatalf("independent verifier refused valid GPU proof: %v", err)
	}
	if worker.Address == verifier.Address {
		t.Fatal("worker and verifier identities match")
	}
	if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(
		bundle.Contract, bundle.Proof, report, digest, verifier.Address,
	); err != nil {
		t.Fatalf("independent signed report rejected: %v", err)
	}
	t.Logf("PASS real sealed CUDA + independent CPU verifier: proof=%s report=%s predictions=%v", bundle.Proof.ID, report.ID, cpu)

	t.Run("wrong-job-rejected", func(t *testing.T) {
		bad := bundle.Contract
		bad.JobID = "wrong-job"
		if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(bad, bundle.Proof, report, digest, verifier.Address); err == nil {
			t.Fatal("cross-job report accepted")
		}
	})
	t.Run("falsified-result-rejected", func(t *testing.T) {
		bad := bundle.Proof
		bad.ResultValues = append([]uint64(nil), bad.ResultValues...)
		bad.ResultValues[0] = (bad.ResultValues[0] + 1) % bad.Task.ColsB
		if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(bundle.Contract, bad, report, digest, verifier.Address); err == nil {
			t.Fatal("falsified result accepted")
		}
	})
	t.Run("unapproved-verifier-rejected", func(t *testing.T) {
		if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(bundle.Contract, bundle.Proof, report, digest, worker.Address); err == nil {
			t.Fatal("unapproved verifier accepted")
		}
	})
	t.Run("altered-report-rejected", func(t *testing.T) {
		bad := report
		bad.OutputHash = strings.Repeat("f", 64)
		if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(bundle.Contract, bundle.Proof, bad, digest, verifier.Address); err == nil {
			t.Fatal("altered verifier report accepted")
		}
	})
}
