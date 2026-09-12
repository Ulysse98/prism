package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"prism/internal/blockchain"
	"prism/internal/usefulwork"
)

type apiMineStartRequest struct {
	Worker string `json:"worker"`
}

type apiMineJob struct {
	ID                string   `json:"id"`
	Worker            string   `json:"worker"`
	WorkerAddress     string   `json:"workerAddress"`
	Task              string   `json:"task"`
	Input             []uint64 `json:"input"`
	InputHash         string   `json:"inputHash"`
	Difficulty        string   `json:"difficulty"`
	Reward            uint64   `json:"reward"`
	Status            string   `json:"status"`
	CreatedAt         string   `json:"createdAt"`
	SourceChainHeight uint64   `json:"sourceChainHeight"`
}

func (api *apiServer) handleMineStart(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiPOSTOnly(writer, request) {
		return
	}

	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		4096,
	)

	var payload apiMineStartRequest

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("invalid mine start request: %w", err),
		)
		return
	}

	payload.Worker = strings.TrimSpace(
		payload.Worker,
	)

	if payload.Worker == "" {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("worker is required"),
		)
		return
	}

	chain, _, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	if len(chain.Blocks) == 0 {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf("blockchain is empty"),
		)
		return
	}

	workerAddress, workerLabel, err :=
		resolveAddress(
			payload.Worker,
			wallets,
		)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			err,
		)
		return
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	// Deterministic small workload derived from the
	// current public chain height. No private key is
	// needed to issue a task.
	base := (lastBlock.Height % 97) + 11

	values := []uint64{
		base,
		base + 3,
		base + 7,
	}

	task, err :=
		usefulwork.NewSumSquaresTask(
			values,
		)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	job := apiMineJob{
		ID:                task.ID,
		Worker:            workerLabel,
		WorkerAddress:     workerAddress,
		Task:              task.Type,
		Input:             task.Values,
		InputHash:         task.InputHash,
		Difficulty:        "LOW",
		Reward:            blockchain.UsefulWorkReward,
		Status:            "READY",
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		SourceChainHeight: lastBlock.Height,
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"job": job,
		},
	)
}

func apiPOSTOnly(
	writer http.ResponseWriter,
	request *http.Request,
) bool {
	if request.Method == http.MethodPost {
		return true
	}

	writer.Header().Set(
		"Allow",
		http.MethodPost,
	)

	apiWriteJSON(
		writer,
		http.StatusMethodNotAllowed,
		map[string]string{
			"error": "method not allowed",
		},
	)

	return false
}
