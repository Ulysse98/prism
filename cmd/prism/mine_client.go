package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"prism/internal/storage"
	"prism/internal/usefulwork"
)

type apiMineStartResponse struct {
	Job apiMineJob `json:"job"`
}

type apiMineSubmitResponse struct {
	Verified    bool            `json:"verified"`
	Reward      uint64          `json:"reward"`
	Block       uint64          `json:"block"`
	TotalSupply uint64          `json:"totalSupply"`
	Proof       apiWorkResponse `json:"proof"`
}

func runMineAPICommand(args []string) {
	flags := flag.NewFlagSet(
		"mine-api",
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
		"local Prism data directory containing the miner wallet",
	)

	taskType := flags.String(
		"task",
		"",
		"useful work task type (sum_squares, dot_product, prime_count, matrix_multiply)",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if flags.NArg() != 1 {
		fmt.Println("Usage:")
		fmt.Println(
			`.\prism.exe mine-api -data .\data Alice`,
		)
		fmt.Println()
		fmt.Println("Optional task selection:")
		fmt.Println(
			`.\prism.exe mine-api -data .\data -task sum_squares Alice`,
		)
		fmt.Println(
			`.\prism.exe mine-api -data .\data -task dot_product Alice`,
		)
		fmt.Println(
			`.\prism.exe mine-api -data .\data -task prime_count Alice`,
		)
		fmt.Println(
			`.\prism.exe mine-api -data .\data -task matrix_multiply Alice`,
		)
		return
	}

	workerName := flags.Arg(0)

	_, _, wallets, err := storage.Load(*dataPath)
	if err != nil {
		fmt.Println("Unable to load miner wallet state:")
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

	var startResponse apiMineStartResponse

	err = mineAPIPost(
		client,
		baseURL+"/mine/start",
		apiMineStartRequest{
			Worker: workerName,
			Task:   *taskType,
		},
		&startResponse,
	)

	if err != nil {
		fmt.Println("Mine start failed:")
		fmt.Println(err)
		return
	}

	job := startResponse.Job

	fmt.Println("=== PRISM MINER ===")
	fmt.Println("Worker:", job.Worker)
	fmt.Println("Address:", job.WorkerAddress)
	fmt.Println("Task:", job.Task)
	fmt.Println("Difficulty:", job.Difficulty)
	fmt.Println("Reward:", job.Reward, "PRISM")
	fmt.Println("Input:", job.Input)

	if len(job.InputB) > 0 {
		fmt.Println("Input B:", job.InputB)
	}

	if job.RowsA != 0 ||
		job.ColsA != 0 ||
		job.ColsB != 0 {

		fmt.Printf(
			"Matrix dimensions: A=%dx%d B=%dx%d\n",
			job.RowsA,
			job.ColsA,
			job.ColsA,
			job.ColsB,
		)
	}

	fmt.Println("Job:", job.ID)
	fmt.Println(
		"Source height:",
		job.SourceChainHeight,
	)
	fmt.Println()

	if job.WorkerAddress != worker.Address {
		fmt.Println("Miner address mismatch.")
		fmt.Println("API:", job.WorkerAddress)
		fmt.Println("Local:", worker.Address)
		return
	}

	task := usefulwork.Task{
		ID:        job.ID,
		Type:      job.Task,
		Values:    job.Input,
		ValuesB:   job.InputB,
		RowsA:     job.RowsA,
		ColsA:     job.ColsA,
		ColsB:     job.ColsB,
		InputHash: job.InputHash,
	}

	fmt.Println("Computing useful work...")

	proof, err := usefulwork.Execute(
		task,
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

	fmt.Println("Score:", proof.Score)
	fmt.Println("Proof ID:", proof.ID)
	fmt.Println(
		"Signature: VERIFIED LOCALLY",
	)
	fmt.Println()

	var submitResponse apiMineSubmitResponse

	err = mineAPIPost(
		client,
		baseURL+"/mine/submit",
		apiMineSubmitRequest{
			JobID:             job.ID,
			SourceChainHeight: job.SourceChainHeight,
			WorkerAddress:     proof.Worker,
			PublicKey:         proof.PublicKey,
			Result:            proof.Result,
			ResultValues:      proof.ResultValues,
			OutputHash:        proof.OutputHash,
			Score:             proof.Score,
			ProofID:           proof.ID,
			Signature:         proof.Signature,
		},
		&submitResponse,
	)

	if err != nil {
		fmt.Println("Mine submit failed:")
		fmt.Println(err)
		return
	}

	fmt.Println("=== MINING CONFIRMED ===")
	fmt.Println(
		"Verified:",
		submitResponse.Verified,
	)
	fmt.Println(
		"Block:",
		submitResponse.Block,
	)
	fmt.Println(
		"Reward:",
		submitResponse.Reward,
		"PRISM",
	)
	fmt.Println(
		"Total supply:",
		submitResponse.TotalSupply,
	)
	fmt.Println(
		"Block hash:",
		submitResponse.Proof.BlockHash,
	)
	fmt.Println(
		"Proof ID:",
		submitResponse.Proof.ProofID,
	)
}

func mineAPIPost(
	client *http.Client,
	url string,
	payload any,
	result any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	responseBody, err := io.ReadAll(
		response.Body,
	)
	if err != nil {
		return err
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {

		return fmt.Errorf(
			"HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(
				string(responseBody),
			),
		)
	}

	if err := json.Unmarshal(
		responseBody,
		result,
	); err != nil {
		return err
	}

	return nil
}
