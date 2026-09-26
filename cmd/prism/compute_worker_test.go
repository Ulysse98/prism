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
