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

type apiMineSubmitRequest struct {
	JobID             string `json:"jobId"`
	SourceChainHeight uint64 `json:"sourceChainHeight"`
	WorkerAddress     string `json:"workerAddress"`
	PublicKey         string `json:"publicKey"`
	Result            uint64 `json:"result"`
	OutputHash        string `json:"outputHash"`
	Score             uint64 `json:"score"`
	ProofID           string `json:"proofId"`
	Signature         string `json:"signature"`
}

func mineTaskForHeight(
	height uint64,
) (usefulwork.Task, error) {
	base := (height % 97) + 11

	return usefulwork.NewSumSquaresTask(
		[]uint64{
			base,
			base + 3,
			base + 7,
		},
	)
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
			fmt.Errorf(
				"invalid mine start request: %w",
				err,
			),
		)
		return
	}

	payload.Worker =
		strings.TrimSpace(payload.Worker)

	if payload.Worker == "" {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("worker is required"),
		)
		return
	}

	chain, _, wallets, err :=
		api.loadState()

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

	task, err :=
		mineTaskForHeight(lastBlock.Height)

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

func (api *apiServer) handleMineSubmit(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiPOSTOnly(writer, request) {
		return
	}

	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		16384,
	)

	var payload apiMineSubmitRequest

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid mine submit request: %w",
				err,
			),
		)
		return
	}

	if payload.JobID == "" ||
		payload.WorkerAddress == "" ||
		payload.PublicKey == "" ||
		payload.OutputHash == "" ||
		payload.ProofID == "" ||
		payload.Signature == "" {

		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"incomplete signed useful work proof",
			),
		)
		return
	}

	api.stateMu.Lock()
	defer api.stateMu.Unlock()

	chain, pos, wallets, err :=
		api.loadState()

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

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	if payload.SourceChainHeight !=
		lastBlock.Height {

		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"stale PoUW job: expected source height %d, got %d",
				lastBlock.Height,
				payload.SourceChainHeight,
			),
		)
		return
	}

	task, err :=
		mineTaskForHeight(
			payload.SourceChainHeight,
		)

	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	if payload.JobID != task.ID {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("invalid PoUW job ID"),
		)
		return
	}

	proof := usefulwork.Proof{
		ID:         payload.ProofID,
		Task:       task,
		Worker:     payload.WorkerAddress,
		PublicKey:  payload.PublicKey,
		Result:     payload.Result,
		OutputHash: payload.OutputHash,
		Score:      payload.Score,
		Signature:  payload.Signature,
	}

	if err := usefulwork.VerifyProof(
		proof,
	); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid signed PoUW proof: %w",
				err,
			),
		)
		return
	}

	proposer, err :=
		pos.SelectProposer(
			lastBlock.Hash,
			lastBlock.Height+1,
		)

	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	block, err :=
		chain.AddBlock(
			nil,
			[]usefulwork.Proof{proof},
			proposer.Address,
			pos,
		)

	if err != nil {
		apiWriteError(
			writer,
			http.StatusConflict,
			err,
		)
		return
	}

	if err := api.saveState(
		chain,
		pos,
		wallets,
	); err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	totalSupply, err :=
		chain.TotalSupply()

	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	responseProof := apiWorkResponse{
		Block: block.Height,
		Worker: walletNameForAddress(
			proof.Worker,
			wallets,
		),
		WorkerAddress: proof.Worker,
		Task:          proof.Task.Type,
		TaskID:        proof.Task.ID,
		Result:        proof.Result,
		Score:         proof.Score,
		Reward:        blockchain.UsefulWorkReward,
		Verified:      true,
		OutputHash:    proof.OutputHash,
		ProofID:       proof.ID,
		BlockHash:     block.Hash,
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"verified":    true,
			"reward":      blockchain.UsefulWorkReward,
			"block":       block.Height,
			"totalSupply": totalSupply,
			"proof":       responseProof,
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
