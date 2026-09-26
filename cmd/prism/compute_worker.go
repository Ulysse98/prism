package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
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

type apiComputeJobResponse struct {
	Job compute.Job `json:"job"`
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

func printComputeWorkerUsage() {
	fmt.Println("Usage:")
	fmt.Println(
		`.\\prism.exe compute-worker -data .\\data Bob <job-id>`,
	)
	fmt.Println(
		`.\\prism.exe compute-worker -data .\\data -auto Bob`,
	)
	fmt.Println()
	fmt.Println("Auto discovery filters:")
	fmt.Println("  -task <type>")
	fmt.Println("  -min-reward <PRISM>")
	fmt.Println("  -limit <1..1000>")
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

	autoMode := flags.Bool(
		"auto",
		false,
		"discover, claim and execute the best OPEN compute job",
	)

	taskFilter := flags.String(
		"task",
		"",
		"optional task type filter used in auto mode",
	)

	minReward := flags.Uint64(
		"min-reward",
		0,
		"minimum marketplace reward used in auto mode",
	)

	limit := flags.Int(
		"limit",
		100,
		"maximum OPEN jobs considered in auto mode",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *limit < 1 || *limit > 1000 {
		fmt.Println(
			"Compute worker limit must be between 1 and 1000.",
		)
		return
	}

	if *autoMode {
		if flags.NArg() != 1 {
			printComputeWorkerUsage()
			return
		}
	} else if flags.NArg() != 2 {
		printComputeWorkerUsage()
		return
	}

	workerName := strings.TrimSpace(
		flags.Arg(0),
	)

	if workerName == "" {
		fmt.Println(
			"Compute worker name cannot be empty.",
		)
		return
	}

	_, _, wallets, err := storage.Load(
		*dataPath,
	)
	if err != nil {
		fmt.Println(
			"Unable to load worker wallet state:",
		)
		fmt.Println(err)
		return
	}

	worker := wallets[workerName]
	if worker == nil {
		fmt.Println(
			"Unknown local worker:",
			workerName,
		)
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	baseURL := strings.TrimRight(
		*apiBase,
		"/",
	)

	status, err := fetchComputeStatus(
		client,
		baseURL+"/status",
	)
	if err != nil {
		fmt.Println(
			"Unable to fetch node status:",
		)
		fmt.Println(err)
		return
	}

	if !status.ChainValid {
		fmt.Println(
			"Node does not report a valid chain.",
		)
		return
	}

	if strings.TrimSpace(status.ChainID) == "" ||
		strings.TrimSpace(status.GenesisHash) == "" {

		fmt.Println(
			"Node status is missing chain identity.",
		)
		return
	}

	var job compute.Job

	if *autoMode {
		discoveryURL, err :=
			buildComputeDiscoveryURL(
				baseURL,
				*taskFilter,
				*minReward,
				*limit,
			)
		if err != nil {
			fmt.Println(
				"Unable to build compute discovery request:",
			)
			fmt.Println(err)
			return
		}

		jobs, err := fetchComputeJobs(
			client,
			discoveryURL,
		)
		if err != nil {
			fmt.Println(
				"Unable to fetch compute marketplace:",
			)
			fmt.Println(err)
			return
		}

		job, err = selectBestComputeJob(
			jobs,
			workerName,
			worker.Address,
		)
		if err != nil {
			fmt.Println(
				"No eligible OPEN compute job:",
			)
			fmt.Println(err)
			return
		}

		fmt.Println(
			"Auto-selected marketplace job:",
			job.ID,
		)
		fmt.Println(
			"Selection reward:",
			job.Reward,
			"PRISM",
		)
		fmt.Println(
			"Selection work units:",
			job.WorkUnits,
		)
		fmt.Println()
	} else {
		jobID := strings.TrimSpace(
			flags.Arg(1),
		)

		if jobID == "" {
			fmt.Println(
				"Compute job ID cannot be empty.",
			)
			return
		}

		job, err = fetchComputeJob(
			client,
			baseURL+
				"/compute/jobs/"+
				jobID,
			jobID,
		)
		if err != nil {
			fmt.Println(
				"Compute job lookup failed:",
			)
			fmt.Println(err)
			return
		}
	}

	if strings.EqualFold(
		job.Requester,
		worker.Address,
	) || strings.EqualFold(
		job.Requester,
		workerName,
	) {
		fmt.Println(
			"Worker cannot execute its own requested compute job.",
		)
		return
	}

	switch job.Status {
	case compute.JobStatusOpen:
		fmt.Println(
			"Claiming OPEN marketplace job...",
		)

		claim, err :=
			compute.SignClaimAuthorization(
				job.ID,
				status.ChainID,
				status.GenesisHash,
				worker,
			)
		if err != nil {
			fmt.Println(
				"Unable to sign compute claim:",
			)
			fmt.Println(err)
			return
		}

		var claimed apiComputeJobResponse

		err = mineAPIPost(
			client,
			baseURL+
				"/compute/jobs/"+
				job.ID+
				"/claim",
			claim,
			&claimed,
		)
		if err != nil {
			fmt.Println(
				"Compute claim failed:",
			)
			fmt.Println(err)
			return
		}

		if claimed.Job.ID != job.ID {
			fmt.Println(
				"Compute claim returned a different job.",
			)
			return
		}

		if claimed.Job.Status !=
			compute.JobStatusClaimed {

			fmt.Println(
				"Compute claim was not confirmed:",
				claimed.Job.Status,
			)
			return
		}

		if claimed.Job.Worker != worker.Address {
			fmt.Println(
				"Compute claim worker mismatch.",
			)
			return
		}

		job = claimed.Job

		fmt.Println(
			"Signed claim: VERIFIED BY NODE",
		)
		fmt.Println()

	case compute.JobStatusClaimed:
		if job.Worker != worker.Address {
			fmt.Println(
				"Compute job is claimed by another worker.",
			)
			fmt.Println(
				"Job worker:",
				job.Worker,
			)
			fmt.Println(
				"Local wallet:",
				worker.Address,
			)
			return
		}

		fmt.Println(
			"Resuming existing worker claim.",
		)
		fmt.Println()

	case compute.JobStatusVerified:
		fmt.Println(
			"Compute job is already VERIFIED.",
		)
		return

	default:
		fmt.Println(
			"Unsupported compute job status:",
			job.Status,
		)
		return
	}

	fmt.Println(
		"=== PRISM COMPUTE WORKER ===",
	)
	fmt.Println(
		"Worker:",
		workerName,
	)
	fmt.Println(
		"Address:",
		worker.Address,
	)
	fmt.Println(
		"Job:",
		job.ID,
	)
	fmt.Println(
		"Task:",
		job.Task.Type,
	)
	fmt.Println(
		"Task ID:",
		job.Task.ID,
	)
	fmt.Println(
		"Work units:",
		job.WorkUnits,
	)
	fmt.Println(
		"Reward:",
		job.Reward,
		"PRISM",
	)
	fmt.Println()
	fmt.Println(
		"Computing useful work...",
	)

	proof, err := usefulwork.ExecuteCompute(
		job.Task,
		job.ID,
		status.ChainID,
		status.GenesisHash,
		worker,
	)
	if err != nil {
		fmt.Println(
			"Useful work execution failed:",
		)
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

	fmt.Println(
		"Score:",
		proof.Score,
	)
	fmt.Println(
		"Output hash:",
		proof.OutputHash,
	)
	fmt.Println(
		"Proof ID:",
		proof.ID,
	)
	fmt.Println(
		"Proof signature: CREATED",
	)
	fmt.Println()
	fmt.Println(
		"Submitting proof...",
	)

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
		fmt.Println(
			"Compute completion failed:",
		)
		fmt.Println(err)
		return
	}

	if err := validateComputeCompletion(
		completed,
		job,
		proof,
	); err != nil {
		fmt.Println(
			"Invalid compute completion response:",
		)
		fmt.Println(err)
		return
	}

	fmt.Println()
	fmt.Println(
		"=== COMPUTE JOB VERIFIED ===",
	)
	fmt.Println(
		"Verified:",
		completed.Verified,
	)
	fmt.Println(
		"Settled:",
		completed.Settled,
	)
	fmt.Println(
		"Status:",
		completed.Job.Status,
	)
	fmt.Println(
		"Job:",
		completed.Job.ID,
	)
	fmt.Println(
		"Worker:",
		completed.Job.Worker,
	)
	fmt.Println(
		"Proof ID:",
		completed.Job.ProofID,
	)
	fmt.Println(
		"Bounty reward:",
		completed.BountyReward,
		"PRISM",
	)
	fmt.Println(
		"Settlement TX:",
		completed.SettlementTxID,
	)
	fmt.Println(
		"Block:",
		completed.Block,
	)
	fmt.Println(
		"Recovered:",
		completed.Recovered,
	)
}

func buildComputeDiscoveryURL(
	baseURL string,
	task string,
	minReward uint64,
	limit int,
) (string, error) {
	endpoint, err := url.Parse(
		strings.TrimRight(
			baseURL,
			"/",
		) + "/compute/jobs",
	)
	if err != nil {
		return "", err
	}

	query := endpoint.Query()

	query.Set(
		"status",
		string(compute.JobStatusOpen),
	)

	task = strings.TrimSpace(task)
	if task != "" {
		query.Set(
			"task",
			task,
		)
	}

	if minReward > 0 {
		query.Set(
			"minReward",
			fmt.Sprintf(
				"%d",
				minReward,
			),
		)
	}

	query.Set(
		"limit",
		fmt.Sprintf(
			"%d",
			limit,
		),
	)

	endpoint.RawQuery = query.Encode()

	return endpoint.String(), nil
}

func computeJobHasBetterValue(
	candidate compute.Job,
	current compute.Job,
) bool {
	var candidateReward big.Int
	var candidateDenominator big.Int
	var currentReward big.Int
	var currentDenominator big.Int

	candidateReward.SetUint64(
		candidate.Reward,
	)
	candidateDenominator.SetUint64(
		current.WorkUnits,
	)
	currentReward.SetUint64(
		current.Reward,
	)
	currentDenominator.SetUint64(
		candidate.WorkUnits,
	)

	var candidateValue big.Int
	var currentValue big.Int

	candidateValue.Mul(
		&candidateReward,
		&candidateDenominator,
	)

	currentValue.Mul(
		&currentReward,
		&currentDenominator,
	)

	switch candidateValue.Cmp(
		&currentValue,
	) {
	case 1:
		return true

	case -1:
		return false
	}

	if candidate.Reward != current.Reward {
		return candidate.Reward >
			current.Reward
	}

	if candidate.WorkUnits !=
		current.WorkUnits {

		return candidate.WorkUnits <
			current.WorkUnits
	}

	return candidate.ID < current.ID
}

func selectBestComputeJob(
	jobs []compute.Job,
	workerName string,
	workerAddress string,
) (compute.Job, error) {
	bestIndex := -1

	for index := range jobs {
		job := jobs[index]

		if job.Status !=
			compute.JobStatusOpen {

			continue
		}

		if job.WorkUnits == 0 {
			continue
		}

		if strings.EqualFold(
			job.Requester,
			workerAddress,
		) || strings.EqualFold(
			job.Requester,
			workerName,
		) {
			continue
		}

		if bestIndex < 0 ||
			computeJobHasBetterValue(
				job,
				jobs[bestIndex],
			) {

			bestIndex = index
		}
	}

	if bestIndex < 0 {
		return compute.Job{},
			fmt.Errorf(
				"no eligible OPEN jobs",
			)
	}

	return jobs[bestIndex], nil
}

func validateComputeCompletion(
	response apiComputeWorkerCompleteResponse,
	job compute.Job,
	proof usefulwork.Proof,
) error {
	if !response.Verified {
		return fmt.Errorf(
			"node did not confirm proof verification",
		)
	}

	if !response.Settled {
		return fmt.Errorf(
			"node did not confirm bounty settlement",
		)
	}

	if response.Job.ID != job.ID {
		return fmt.Errorf(
			"completion returned a different job",
		)
	}

	if response.Job.Status !=
		compute.JobStatusVerified {

		return fmt.Errorf(
			"completion job is not VERIFIED",
		)
	}

	if response.Job.Worker != proof.Worker {
		return fmt.Errorf(
			"completion worker mismatch",
		)
	}

	if response.Job.ProofID != proof.ID {
		return fmt.Errorf(
			"completion proof ID mismatch",
		)
	}

	if response.BountyReward != job.Reward {
		return fmt.Errorf(
			"completion bounty mismatch",
		)
	}

	if strings.TrimSpace(
		response.SettlementTxID,
	) == "" {
		return fmt.Errorf(
			"completion settlement transaction is missing",
		)
	}

	return nil
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
		io.LimitReader(
			response.Body,
			1<<20,
		),
	)
	if err != nil {
		return apiStatusResponse{}, err
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		return apiStatusResponse{},
			fmt.Errorf(
				"HTTP %d: %s",
				response.StatusCode,
				strings.TrimSpace(
					string(body),
				),
			)
	}

	var payload apiStatusResponse

	if err := json.Unmarshal(
		body,
		&payload,
	); err != nil {
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

		return nil,
			fmt.Errorf(
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

func fetchComputeJob(
	client *http.Client,
	url string,
	expectedID string,
) (compute.Job, error) {
	response, err := client.Get(url)
	if err != nil {
		return compute.Job{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(
			response.Body,
			1<<20,
		),
	)
	if err != nil {
		return compute.Job{}, err
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		return compute.Job{},
			fmt.Errorf(
				"HTTP %d: %s",
				response.StatusCode,
				strings.TrimSpace(
					string(body),
				),
			)
	}

	var payload apiComputeJobResponse

	if err := json.Unmarshal(
		body,
		&payload,
	); err != nil {
		return compute.Job{}, err
	}

	if payload.Job.ID != expectedID {
		return compute.Job{},
			fmt.Errorf(
				"compute job response ID mismatch",
			)
	}

	return payload.Job, nil
}
