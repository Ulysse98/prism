package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"prism/internal/compute"
	"prism/internal/usefulwork"
)

// These tests use the real CLI handler and the real ephemeral HTTP API/node
// from the already validated isolated marketplace E2E lab.
func TestGPUV063ActualCLITwoJobsReuseSessionLab(t *testing.T) {
	bin := v063MarketplaceLabGPU(t)
	l := newV063MarketLab(t)
	owner := &gpuCLISessionV063{binary: bin}
	defer func() {
		if err := owner.Close(); err != nil {
			t.Errorf("closing GPU session: %v", err)
		}
	}()
	var settled []compute.Job
	for i := 0; i < 2; i++ {
		task := v063MarketTask(t, int64(i))
		job, err := l.market.Create(task, "Alice", 1, uint64(63800+i))
		if err != nil {
			t.Fatal(err)
		}
		h0, s0, _, b0 := l.chainSnapshot(t)
		result := runComputeWorkerCommandWithGPUV063([]string{
			"-api", l.baseURL, "-data", l.dataDir, "-compute-backend", "gpu-session", "-gpu-bin", bin, "Bob", job.ID,
		}, owner)
		if !result.Processed || !result.Completed || result.JobID != job.ID {
			t.Fatalf("CLI job %d failed: %+v", i, result)
		}
		final, err := l.market.Get(job.ID)
		if err != nil || final.Status != compute.JobStatusVerified || final.ProofID == "" {
			t.Fatalf("CLI job not VERIFIED: %+v / %v", final, err)
		}
		h1, s1, _, b1 := l.chainSnapshot(t)
		if h1 != h0+1 || b1 <= b0 {
			t.Fatalf("invalid settlement h %d->%d supply %d->%d balance %d->%d", h0, h1, s0, s1, b0, b1)
		}
		settled = append(settled, final)
	}
	if owner.starts != 1 || owner.supervisor == nil || owner.supervisor.served != 2 {
		t.Fatalf("GPU process not reused: starts=%d supervisor=%v", owner.starts, owner.supervisor)
	}
	for _, job := range settled {
		if job.Status != compute.JobStatusVerified {
			t.Fatalf("job lost VERIFIED status: %s", job.ID)
		}
	}
	t.Log("PASS: actual CLI, signed claim, two jobs, one CUDA process, two unique bounty settlements")
}

func TestGPUV063ActualCLIFailureNoPaymentAndRecoveryLab(t *testing.T) {
	realBin := v063MarketplaceLabGPU(t)
	for _, tc := range []struct{ name, script string }{
		{"crash", "#!/bin/sh\nIFS= read -r request || exit 1\nexit 17\n"},
		{"timeout", "#!/bin/sh\nIFS= read -r request || exit 1\nsleep 30 &\nwait\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newV063MarketLab(t)
			job, err := l.market.Create(v063MarketTask(t, 4), "Alice", 1, 63811)
			if err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "fake-session")
			if err := os.WriteFile(fake, []byte(tc.script), 0700); err != nil {
				t.Fatal(err)
			}
			owner := &gpuCLISessionV063{binary: fake, requestTimeout: 350 * time.Millisecond}
			h0, s0, a0, b0 := l.chainSnapshot(t)
			got := runComputeWorkerCommandWithGPUV063([]string{
				"-api", l.baseURL, "-data", l.dataDir, "-compute-backend", "gpu-session", "-gpu-bin", fake, "Bob", job.ID,
			}, owner)
			_ = owner.Close()
			if got.Completed {
				t.Fatalf("unexpected GPU completion %+v", got)
			}
			claimed, err := l.market.Get(job.ID)
			if err != nil || claimed.Status != compute.JobStatusClaimed || strings.TrimSpace(claimed.Worker) == "" {
				t.Fatalf("job not CLAIMED after GPU error: %+v / %v", claimed, err)
			}
			h1, s1, a1, b1 := l.chainSnapshot(t)
			if h0 != h1 || s0 != s1 || a0 != a1 || b0 != b1 {
				t.Fatal("GPU failure changed chain or balances")
			}
			recovered := &gpuCLISessionV063{binary: realBin}
			defer func() { _ = recovered.Close() }()
			ok := runComputeWorkerCommandWithGPUV063([]string{
				"-api", l.baseURL, "-data", l.dataDir, "-compute-backend", "gpu-session", "-gpu-bin", realBin, "Bob", job.ID,
			}, recovered)
			if !ok.Completed {
				t.Fatalf("fresh real CUDA CLI session failed: %+v", ok)
			}
			h2, _, _, b2 := l.chainSnapshot(t)
			if h2 != h1+1 || b2 <= b1 {
				t.Fatalf("recovery settlement missing h %d->%d", h1, h2)
			}
			t.Log("PASS: actual CLI failure no payout; claimed job recovered exactly once")
		})
	}
}

func TestGPUV063ActualCLIUnsupportedNoClaimLab(t *testing.T) {
	bin := v063MarketplaceLabGPU(t)
	l := newV063MarketLab(t)
	// Valid on the marketplace but incompatible with the 3x3 GPU contract.
	task, err := usefulwork.NewMLInferenceQuantizedTask(2, 2, 2,
		[]int64{1, 2, 3, 4}, []int64{1, 0, 0, 1}, []int64{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	job, err := l.market.Create(task, "Alice", 1, 63830)
	if err != nil {
		t.Fatal(err)
	}
	result := runComputeWorkerCommandWithGPUV063([]string{
		"-api", l.baseURL, "-data", l.dataDir, "-compute-backend", "gpu-session", "-gpu-bin", bin, "Bob", job.ID,
	}, &gpuCLISessionV063{binary: bin})
	if result.Completed {
		t.Fatalf("unsupported job completed %+v", result)
	}
	final, err := l.market.Get(job.ID)
	if err != nil || final.Status != compute.JobStatusOpen {
		t.Fatalf("unsupported job was claimed: %+v %v", final, err)
	}
	t.Log("PASS: actual CLI refuses incompatible GPU task before claim")
}

func TestGPUV063AutoCLITwoJobsReuseProcessLab(t *testing.T) {
	bin := v063MarketplaceLabGPU(t)
	l := newV063MarketLab(t)
	owner := &gpuCLISessionV063{binary: bin}
	defer func() { _ = owner.Close() }()
	// OPEN, unclaimed jobs. The auto-cycle must claim, compute and settle each.
	for i := 0; i < 2; i++ {
		_, err := l.market.Create(v063MarketTask(t, int64(i)), "Alice", 1, uint64(63880+i))
		if err != nil {
			t.Fatal(err)
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	attempted := make(map[string]struct{})
	stats := autonomousWorkerStats{StartedAt: time.Now()}
	for i := 0; i < 2; i++ {
		stats.Cycles++
		if !runGPUV063AutoCycle(client, l.baseURL, l.dataDir, "Bob", l.bob.Address, "", 0, 100,
			owner, attempted, &stats) {
			t.Fatalf("auto-cycle %d failed to process a job", i)
		}
	}
	if stats.CompletedJobs != 2 || owner.starts != 1 || owner.supervisor == nil || owner.supervisor.served != 2 {
		t.Fatalf("auto CUDA process reuse failed: %+v starts=%d", stats, owner.starts)
	}
	h, _, _, _ := l.chainSnapshot(t)
	if h < 3 {
		t.Fatalf("missing two settlement blocks: height %d", h)
	}
	t.Log("PASS: actual autonomous selection + two signed claims/settlements, one CUDA process")
}

func TestGPUV063AutoCLIRejectUnsupportedBeforeClaimLab(t *testing.T) {
	bin := v063MarketplaceLabGPU(t)
	l := newV063MarketLab(t)
	task, err := usefulwork.NewMLInferenceQuantizedTask(2, 2, 2,
		[]int64{1, 2, 3, 4}, []int64{1, 0, 0, 1}, []int64{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	incompatible, err := l.market.Create(task, "Alice", 8, 63900)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := l.market.Create(v063MarketTask(t, 0), "Alice", 1, 63901)
	if err != nil {
		t.Fatal(err)
	}
	owner := &gpuCLISessionV063{binary: bin}
	defer func() { _ = owner.Close() }()
	stats := autonomousWorkerStats{StartedAt: time.Now()}
	attempted := make(map[string]struct{})
	if !runGPUV063AutoCycle(l.server.Client(), l.baseURL, l.dataDir, "Bob", l.bob.Address, "", 0, 100,
		owner, attempted, &stats) {
		t.Fatal("valid job was not processed")
	}
	bad, err := l.market.Get(incompatible.ID)
	if err != nil || bad.Status != compute.JobStatusOpen {
		t.Fatalf("incompatible job was claimed: %+v %v", bad, err)
	}
	good, err := l.market.Get(valid.ID)
	if err != nil || good.Status != compute.JobStatusVerified {
		t.Fatalf("valid job not verified: %+v %v", good, err)
	}
	t.Log("PASS: autonomous mode ignores high-reward incompatible jobs without claiming")
}
