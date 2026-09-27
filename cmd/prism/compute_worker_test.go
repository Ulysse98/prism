package main

import (
	"net/url"
	"testing"

	"prism/internal/compute"
)

func TestSelectBestComputeJobByRewardPerWorkUnit(
	t *testing.T,
) {
	jobs := []compute.Job{
		{
			ID:        "a",
			Requester: "Alice",
			Reward:    10,
			WorkUnits: 5,
			Status:    compute.JobStatusOpen,
		},
		{
			ID:        "b",
			Requester: "Alice",
			Reward:    9,
			WorkUnits: 3,
			Status:    compute.JobStatusOpen,
		},
		{
			ID:        "c",
			Requester: "Alice",
			Reward:    100,
			WorkUnits: 1,
			Status:    compute.JobStatusClaimed,
		},
	}

	selected, err := selectBestComputeJob(
		jobs,
		"Bob",
		"prism_bob",
	)
	if err != nil {
		t.Fatal(err)
	}

	if selected.ID != "b" {
		t.Fatalf(
			"unexpected selected job: %s",
			selected.ID,
		)
	}
}

func TestSelectBestComputeJobTieBreaksDeterministically(
	t *testing.T,
) {
	jobs := []compute.Job{
		{
			ID:        "b",
			Requester: "Alice",
			Reward:    10,
			WorkUnits: 5,
			Status:    compute.JobStatusOpen,
		},
		{
			ID:        "c",
			Requester: "Alice",
			Reward:    20,
			WorkUnits: 10,
			Status:    compute.JobStatusOpen,
		},
		{
			ID:        "a",
			Requester: "Alice",
			Reward:    20,
			WorkUnits: 10,
			Status:    compute.JobStatusOpen,
		},
	}

	selected, err := selectBestComputeJob(
		jobs,
		"Bob",
		"prism_bob",
	)
	if err != nil {
		t.Fatal(err)
	}

	if selected.ID != "a" {
		t.Fatalf(
			"unexpected selected job: %s",
			selected.ID,
		)
	}
}

func TestSelectBestComputeJobSkipsSelfRequested(
	t *testing.T,
) {
	jobs := []compute.Job{
		{
			ID:        "self-address",
			Requester: "prism_bob",
			Reward:    100,
			WorkUnits: 1,
			Status:    compute.JobStatusOpen,
		},
		{
			ID:        "self-name",
			Requester: "bob",
			Reward:    90,
			WorkUnits: 1,
			Status:    compute.JobStatusOpen,
		},
		{
			ID:        "eligible",
			Requester: "Alice",
			Reward:    5,
			WorkUnits: 5,
			Status:    compute.JobStatusOpen,
		},
	}

	selected, err := selectBestComputeJob(
		jobs,
		"Bob",
		"prism_bob",
	)
	if err != nil {
		t.Fatal(err)
	}

	if selected.ID != "eligible" {
		t.Fatalf(
			"unexpected selected job: %s",
			selected.ID,
		)
	}
}

func TestBuildComputeDiscoveryURL(
	t *testing.T,
) {
	raw, err := buildComputeDiscoveryURL(
		"http://127.0.0.1:8080/api/v1",
		"ml_inference_batch",
		25,
		20,
	)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	query := parsed.Query()

	if query.Get("status") != "OPEN" {
		t.Fatalf(
			"unexpected status filter: %s",
			query.Get("status"),
		)
	}

	if query.Get("task") !=
		"ml_inference_batch" {

		t.Fatalf(
			"unexpected task filter: %s",
			query.Get("task"),
		)
	}

	if query.Get("minReward") != "25" {
		t.Fatalf(
			"unexpected reward filter: %s",
			query.Get("minReward"),
		)
	}

	if query.Get("limit") != "20" {
		t.Fatalf(
			"unexpected limit filter: %s",
			query.Get("limit"),
		)
	}
}

func TestBuildClaimedComputeDiscoveryURL(
	t *testing.T,
) {
	raw, err := buildClaimedComputeDiscoveryURL(
		"http://127.0.0.1:8080/api/v1",
		"prism_bob",
		25,
	)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	query := parsed.Query()

	if query.Get("status") != "CLAIMED" {
		t.Fatalf(
			"unexpected status: %s",
			query.Get("status"),
		)
	}

	if query.Get("worker") != "prism_bob" {
		t.Fatalf(
			"unexpected worker: %s",
			query.Get("worker"),
		)
	}

	if query.Get("limit") != "25" {
		t.Fatalf(
			"unexpected limit: %s",
			query.Get("limit"),
		)
	}
}

func TestSelectRecoverableClaimedJob(
	t *testing.T,
) {
	jobs := []compute.Job{
		{
			ID:     "b",
			Status: compute.JobStatusClaimed,
			Worker: "prism_bob",
		},
		{
			ID:     "a",
			Status: compute.JobStatusClaimed,
			Worker: "prism_bob",
		},
		{
			ID:     "other",
			Status: compute.JobStatusClaimed,
			Worker: "prism_charlie",
		},
	}

	attempted := map[string]struct{}{
		"a": {},
	}

	job, found :=
		selectRecoverableClaimedJob(
			jobs,
			"prism_bob",
			attempted,
		)

	if !found {
		t.Fatal(
			"expected recoverable claimed job",
		)
	}

	if job.ID != "b" {
		t.Fatalf(
			"unexpected recovered job: %s",
			job.ID,
		)
	}

	attempted["b"] = struct{}{}

	_, found =
		selectRecoverableClaimedJob(
			jobs,
			"prism_bob",
			attempted,
		)

	if found {
		t.Fatal(
			"expected all Bob claims to be quarantined",
		)
	}
}

func TestAutonomousWorkerStatsAccumulateRuntimeMetrics(
	t *testing.T,
) {
	stats := autonomousWorkerStats{}

	recordAutonomousComputeIteration(
		&stats,
		autonomousComputeIterationResult{
			Processed:    true,
			Completed:    true,
			JobID:        "completed-job",
			BountyReward: 25,
		},
	)

	recordAutonomousComputeIteration(
		&stats,
		autonomousComputeIterationResult{
			Processed: true,
			JobID:     "failed-job",
		},
	)

	// No marketplace job was found:
	// this must not count as a failure.
	recordAutonomousComputeIteration(
		&stats,
		autonomousComputeIterationResult{},
	)

	if stats.CompletedJobs != 1 {
		t.Fatalf(
			"unexpected completed jobs: %d",
			stats.CompletedJobs,
		)
	}

	if stats.FailedJobs != 1 {
		t.Fatalf(
			"unexpected failed jobs: %d",
			stats.FailedJobs,
		)
	}

	if stats.BountyEarned != 25 {
		t.Fatalf(
			"unexpected bounty earned: %d",
			stats.BountyEarned,
		)
	}
}
