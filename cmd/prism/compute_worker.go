package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"prism/internal/compute"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

type apiComputeJobsResponse struct {
	Jobs []compute.Job `json:"jobs"`
}

type apiComputeWorkerCompleteResponse struct {
	Verified       bool        `json:"verified"`
	Settled        bool        `json:"settled"`
	Job            compute.Job `json:"job"`
	BountyReward   uint64      `json:"bountyReward"`
	SettlementTxID string      `json:"settlementTxId"`
	Block          uint64      `json:"block"`
	Recovered      bool        `json:"recovered"`
}

func runComputeWorkerCommand(args []string) {
	flags := flag.NewFlagSet(
		"compute-worker",
		flag.ContinueOnError,
	)

	flags.SetOutput(os.Stdout)

	apiBase := flags.String(
		"api",
		"http://127.0.0.1:8080/api/v1",
		"Prism HTTP API base URL",
	)

	dataPath := flags.String(
		"data",
		"data",
		"local Prism data directory containing the worker wallet",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if flags.NArg() != 2 {
		fmt.Println("Usage:")
		fmt.Println(
			`.\\prism.exe compute-worker -data .\\data Bob <job-id>`,
		)
		return
	}

	workerName := flags.Arg(0)
	jobID := strings.TrimSpace(flags.Arg(1))

	_, _, wallets, err := storage.Load(*dataPath)
	if err != nil {
		fmt.Println("Unable to load worker wallet state:")
		fmt.Println(err)
		return
	}

	worker := wallets[workerName]
	if worker == nil {
		fmt.Println("Unknown local worker:", workerName)
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	baseURL := strings.TrimRight(
		*apiBase,
		"/",
	)

	jobs, err := fetchComputeJobs(
		client,
		baseURL+"/compute/jobs",
	)
	if err != nil {
		fmt.Println("Unable to fetch compute marketplace:")
		fmt.Println(err)
		return
	}

	job, err := findComputeJob(
		jobs,
		jobID,
	)
	if err != nil {
		fmt.Println("Compute job lookup failed:")
		fmt.Println(err)
		return
	}

	if job.Status != compute.JobStatusClaimed {
		fmt.Println(
			"Compute job is not CLAIMED:",
			job.Status,
		)
		return
	}

	if job.Worker != worker.Address {
		fmt.Println("Compute job worker mismatch.")
		fmt.Println("Job worker:", job.Worker)
		fmt.Println("Local wallet:", worker.Address)
		return
	}

	status, err := fetchComputeStatus(
		client,
		baseURL+"/status",
	)
	if err != nil {
		fmt.Println("Unable to fetch node status:")
		fmt.Println(err)
		return
	}
	if !status.ChainValid {
		fmt.Println("Node does not report a valid chain.")
		return
	}
	if status.ChainID == "" || status.GenesisHash == "" {
		fmt.Println("Node status is missing chain identity.")
		return
	}

	fmt.Println("=== PRISM COMPUTE WORKER ===")
	fmt.Println("Worker:", workerName)
	fmt.Println("Address:", worker.Address)
	fmt.Println("Job:", job.ID)
	fmt.Println("Task:", job.Task.Type)
	fmt.Println("Task ID:", job.Task.ID)
	fmt.Println("Work units:", job.WorkUnits)
	fmt.Println("Reward:", job.Reward, "PRISM")
	fmt.Println()
	fmt.Println("Computing useful work...")

	proof, err := usefulwork.ExecuteCompute(
		job.Task,
		job.ID,
		status.ChainID,
		status.GenesisHash,
		worker,
	)
	if err != nil {
		fmt.Println("Useful work execution failed:")
		fmt.Println(err)
		return
	}

	if len(proof.ResultValues) > 0 {
		fmt.Println(
			"Result:",
			proof.ResultValues,
		)
	} else {
		fmt.Println(
			"Result:",
			proof.Result,
		)
	}

	fmt.Println("Score:", proof.Score)
	fmt.Println("Output hash:", proof.OutputHash)
	fmt.Println("Proof ID:", proof.ID)
	fmt.Println("Signature: CREATED")
	fmt.Println()
	fmt.Println("Submitting proof...")

	var completed apiComputeWorkerCompleteResponse

	err = mineAPIPost(
		client,
		baseURL+
			"/compute/jobs/"+
			job.ID+
			"/complete",
		apiComputeCompleteJobRequest{
			Proof: proof,
		},
		&completed,
	)
	if err != nil {
		fmt.Println("Compute completion failed:")
		fmt.Println(err)
		return
	}

	fmt.Println()
	fmt.Println("=== COMPUTE JOB VERIFIED ===")
	fmt.Println("Verified:", completed.Verified)
	fmt.Println("Status:", completed.Job.Status)
	fmt.Println("Job:", completed.Job.ID)
	fmt.Println("Worker:", completed.Job.Worker)
	fmt.Println("Proof ID:", completed.Job.ProofID)
	fmt.Println("Bounty reward:", completed.BountyReward, "PRISM")
	fmt.Println("Settlement TX:", completed.SettlementTxID)
	fmt.Println("Block:", completed.Block)
	fmt.Println("Recovered:", completed.Recovered)
}

func fetchComputeStatus(
	client *http.Client,
	url string,
) (apiStatusResponse, error) {
	response, err := client.Get(url)
	if err != nil {
		return apiStatusResponse{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(response.Body, 1<<20),
	)
	if err != nil {
		return apiStatusResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return apiStatusResponse{}, fmt.Errorf(
			"HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var payload apiStatusResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return apiStatusResponse{}, err
	}
	return payload, nil
}

func fetchComputeJobs(
	client *http.Client,
	url string,
) ([]compute.Job, error) {

	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(
			response.Body,
			1<<20,
		),
	)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		return nil, fmt.Errorf(
			"HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(
				string(body),
			),
		)
	}

	var payload apiComputeJobsResponse

	if err := json.Unmarshal(
		body,
		&payload,
	); err != nil {
		return nil, err
	}

	return payload.Jobs, nil
}

func findComputeJob(
	jobs []compute.Job,
	jobID string,
) (compute.Job, error) {

	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return compute.Job{},
			fmt.Errorf(
				"compute job ID cannot be empty",
			)
	}

	for _, job := range jobs {
		if job.ID == jobID {
			return job, nil
		}
	}

	return compute.Job{},
		fmt.Errorf(
			"compute job not found: %s",
			jobID,
		)
}
