package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"prism/internal/compute"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// gpuCLISessionV063 belongs to one CLI loop. It is never shared between worker
// processes or used by the default CPU/GPU-v0.62 backends.
type gpuCLISessionV063 struct {
	binary         string
	supervisor     *gpuSessionSupervisorV063
	requestTimeout time.Duration
	sequence       int
	starts         int
}

func (s *gpuCLISessionV063) Close() error {
	if s == nil || s.supervisor == nil {
		return nil
	}
	old := s.supervisor
	s.supervisor = nil
	return old.Close()
}

func (s *gpuCLISessionV063) Execute(task usefulwork.Task, jobID, chainID, genesisHash string, worker *wallet.Wallet) (usefulwork.Proof, error) {
	if s == nil {
		return usefulwork.Proof{}, fmt.Errorf("missing GPU session owner")
	}
	if err := validateGPUComputeTaskV062(task); err != nil {
		return usefulwork.Proof{}, err
	}
	if err := validateGPUWorkerBinaryV062(s.binary); err != nil {
		return usefulwork.Proof{}, err
	}
	// The worker enforces a 64-request cap and a 45-second process lifetime.
	// Rotate before sending another request. A killed session cannot be reused.
	if s.supervisor != nil && (s.supervisor.IsDead() || s.supervisor.life.Err() != nil || s.supervisor.served >= 64) {
		_ = s.Close()
	}
	if s.supervisor == nil {
		timeout := s.requestTimeout
		if timeout <= 0 {
			timeout = 20 * time.Second
		}
		next, err := startGPUV063Supervisor(s.binary, timeout)
		if err != nil {
			return usefulwork.Proof{}, err
		}
		s.supervisor = next
		s.starts++
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	values, err := s.supervisor.Execute(ctx, task, s.sequence)
	if err != nil {
		_ = s.Close() // poison and discard; never submit a proof on GPU error
		return usefulwork.Proof{}, err
	}
	s.sequence++
	return usefulwork.BuildComputeMLProofV2FromResult(task, jobID, chainID, genesisHash, worker, values)
}

// One autonomous scan is independently testable against a temporary HTTP node.
// An incompatible GPU task is NEVER claimed; a failure never submits a proof.
func runGPUV063AutoCycle(client *http.Client, baseURL, dataPath, workerName, workerAddress, taskFilter string,
	minReward uint64, limit int, owner *gpuCLISessionV063, attemptedClaimed map[string]struct{}, stats *autonomousWorkerStats) bool {
	var job compute.Job
	var found bool
	claimedURL, err := buildClaimedComputeDiscoveryURL(baseURL, workerAddress, limit)
	if err != nil {
		fmt.Println("GPU claimed-job URL rejected:", err)
		return false
	}
	claimedJobs, err := fetchComputeJobs(client, claimedURL)
	if err != nil {
		fmt.Println("GPU claimed-job scan failed:", err)
		return false // fail closed: do not claim new work while recovery state is unknown
	}
	eligibleClaimed := make([]compute.Job, 0, len(claimedJobs))
	for _, candidate := range claimedJobs {
		if validateGPUComputeTaskV062(candidate.Task) == nil {
			eligibleClaimed = append(eligibleClaimed, candidate)
		}
	}
	job, found = selectRecoverableClaimedJob(eligibleClaimed, workerAddress, attemptedClaimed)
	if found {
		stats.RecoveredClaims++
	} else {
		stats.OpenScans++
		discoveryURL, buildErr := buildComputeDiscoveryURL(baseURL, taskFilter, minReward, limit)
		if buildErr != nil {
			fmt.Println("GPU discovery URL rejected:", buildErr)
			return false
		}
		jobs, getErr := fetchComputeJobs(client, discoveryURL)
		if getErr != nil {
			fmt.Println("GPU marketplace scan failed:", getErr)
			return false
		}
		eligible := make([]compute.Job, 0, len(jobs))
		for _, candidate := range jobs {
			if validateGPUComputeTaskV062(candidate.Task) == nil {
				eligible = append(eligible, candidate)
			}
		}
		job, err = selectBestComputeJob(eligible, workerName, workerAddress)
		if err != nil {
			return false
		}
	}
	args := []string{"-api", baseURL, "-data", dataPath, "-compute-backend", "gpu-session",
		"-gpu-bin", owner.binary, workerName, job.ID}
	result := runComputeWorkerCommandWithGPUV063(args, owner)
	recordAutonomousComputeIteration(stats, result)
	if job.Status == compute.JobStatusClaimed && !result.Completed {
		attemptedClaimed[job.ID] = struct{}{}
	}
	if result.Completed {
		delete(attemptedClaimed, job.ID)
	}
	return result.Processed
}

// Autonomous gpu-session mode: the SAME session owner is used for every cycle.
// Default CPU and GPU-v0.62 loops remain unmodified and disabled backends remain so.
func runContinuousGPUV063Worker(apiBase, dataPath, workerName, workerAddress, taskFilter string,
	minReward uint64, limit int, poll time.Duration, owner *gpuCLISessionV063) {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupts)
	defer func() {
		if err := owner.Close(); err != nil {
			fmt.Println("GPU session shutdown:", err)
		}
	}()
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := strings.TrimRight(apiBase, "/")
	attemptedClaimed := make(map[string]struct{})
	stats := autonomousWorkerStats{StartedAt: time.Now()}
	fmt.Println("=== PRISM GPU v0.63 PERSISTENT WORKER (EXPERIMENTAL) ===")
	for {
		select {
		case <-interrupts:
			printAutonomousWorkerSummary(stats)
			return
		default:
		}
		stats.Cycles++
		_ = runGPUV063AutoCycle(client, baseURL, dataPath, workerName, workerAddress,
			taskFilter, minReward, limit, owner, attemptedClaimed, &stats)
		if !waitForNextComputePollOrStop(poll, interrupts) {
			printAutonomousWorkerSummary(stats)
			return
		}
	}
}
