package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"prism/internal/compute"
	"prism/internal/usefulwork"
)

type apiComputeCreateJobRequest struct {
	Task       *usefulwork.Task `json:"task,omitempty"`
	Type       string           `json:"type,omitempty"`
	Values     []uint64         `json:"values,omitempty"`
	ValuesB    []uint64         `json:"valuesB,omitempty"`
	RowsA      uint64           `json:"rowsA,omitempty"`
	ColsA      uint64           `json:"colsA,omitempty"`
	ColsB      uint64           `json:"colsB,omitempty"`
	Rows       uint64           `json:"rows,omitempty"`
	Cols       uint64           `json:"cols,omitempty"`
	KernelSize uint64           `json:"kernelSize,omitempty"`
	Requester  string           `json:"requester"`
	Reward     uint64           `json:"reward"`
	Nonce      uint64           `json:"nonce"`
}

func (payload apiComputeCreateJobRequest) buildTask() (
	usefulwork.Task,
	error,
) {
	if payload.Task != nil {
		hasInlineTask :=
			strings.TrimSpace(payload.Type) != "" ||
				len(payload.Values) != 0 ||
				len(payload.ValuesB) != 0 ||
				payload.RowsA != 0 ||
				payload.ColsA != 0 ||
				payload.ColsB != 0 ||
				payload.Rows != 0 ||
				payload.Cols != 0 ||
				payload.KernelSize != 0

		if hasInlineTask {
			return usefulwork.Task{}, fmt.Errorf(
				"compute request cannot combine task with inline task fields",
			)
		}

		if err := usefulwork.ValidateTask(*payload.Task); err != nil {
			return usefulwork.Task{}, err
		}

		return *payload.Task, nil
	}

	taskType := strings.TrimSpace(payload.Type)

	switch taskType {
	case usefulwork.TaskTypeSumSquares:
		return usefulwork.NewSumSquaresTask(
			payload.Values,
		)

	case usefulwork.TaskTypeDotProduct:
		return usefulwork.NewDotProductTask(
			payload.Values,
			payload.ValuesB,
		)

	case usefulwork.TaskTypePrimeCount:
		return usefulwork.NewPrimeCountTask(
			payload.Values,
		)

	case usefulwork.TaskTypeMatrixMultiply:
		return usefulwork.NewMatrixMultiplyTask(
			payload.RowsA,
			payload.ColsA,
			payload.ColsB,
			payload.Values,
			payload.ValuesB,
		)

	case usefulwork.TaskTypeImageConvolution:
		return usefulwork.NewImageConvolutionTask(
			payload.Rows,
			payload.Cols,
			payload.KernelSize,
			payload.Values,
			payload.ValuesB,
		)

	case "":
		return usefulwork.Task{}, fmt.Errorf(
			"compute task type cannot be empty",
		)

	default:
		return usefulwork.Task{}, fmt.Errorf(
			"unsupported compute task type: %s",
			taskType,
		)
	}
}

type apiComputeClaimJobRequest struct {
	Worker string `json:"worker"`
}

type apiComputeCompleteJobRequest struct {
	Proof usefulwork.Proof `json:"proof"`
}

func (api *apiServer) createFundedComputeJob(
	task usefulwork.Task,
	requester string,
	reward uint64,
	nonce uint64,
) (compute.Job, int, error) {

	if api == nil {
		return compute.Job{},
			http.StatusInternalServerError,
			fmt.Errorf("API server cannot be nil")
	}

	if api.computeMarket == nil {
		return compute.Job{},
			http.StatusInternalServerError,
			fmt.Errorf("compute marketplace is unavailable")
	}

	// Serialise funding check + marketplace creation.
	// This prevents two concurrent HTTP requests from both
	// observing the same unreserved balance.
	api.stateMu.Lock()
	defer api.stateMu.Unlock()

	chain, _, wallets, err := api.loadState()
	if err != nil {
		return compute.Job{},
			http.StatusInternalServerError,
			err
	}

	requesterName, requesterWallet, err :=
		resolveLocalWallet(
			requester,
			wallets,
		)
	if err != nil {
		return compute.Job{},
			http.StatusBadRequest,
			fmt.Errorf(
				"compute requester must be a local wallet: %w",
				err,
			)
	}

	available, err :=
		chain.AvailableBalanceOf(
			requesterWallet.Address,
		)
	if err != nil {
		return compute.Job{},
			http.StatusInternalServerError,
			err
	}

	// Historical jobs may store the requester as its local
	// wallet name or directly as its Prism address.
	reservedByName, err :=
		api.computeMarket.ReservedRewardFor(
			requesterName,
		)
	if err != nil {
		return compute.Job{},
			http.StatusInternalServerError,
			err
	}

	reservedByAddress, err :=
		api.computeMarket.ReservedRewardFor(
			requesterWallet.Address,
		)
	if err != nil {
		return compute.Job{},
			http.StatusInternalServerError,
			err
	}

	if reservedByName >
		math.MaxUint64-reservedByAddress {

		return compute.Job{},
			http.StatusInternalServerError,
			fmt.Errorf(
				"compute funded reward overflow",
			)
	}

	reserved := reservedByName + reservedByAddress

	if reserved > available ||
		reward > available-reserved {

		return compute.Job{},
			http.StatusBadRequest,
			fmt.Errorf(
				"insufficient funded compute balance: available %d PRISM, reserved %d PRISM, requested %d PRISM",
				available,
				reserved,
				reward,
			)
	}

	job, err := api.computeMarket.Create(
		task,
		requester,
		reward,
		nonce,
	)
	if err != nil {
		return compute.Job{},
			http.StatusBadRequest,
			err
	}

	return job, http.StatusCreated, nil
}

func (api *apiServer) handleComputeJobs(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if api.computeMarket == nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf("compute marketplace is unavailable"),
		)
		return
	}

	switch request.Method {
	case http.MethodGet:
		apiWriteJSON(
			writer,
			http.StatusOK,
			map[string]any{
				"jobs": api.computeMarket.List(),
			},
		)

	case http.MethodPost:
		request.Body = http.MaxBytesReader(
			writer,
			request.Body,
			65536,
		)

		var payload apiComputeCreateJobRequest

		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&payload); err != nil {
			apiWriteError(
				writer,
				http.StatusBadRequest,
				fmt.Errorf(
					"invalid compute job request: %w",
					err,
				),
			)
			return
		}

		task, err := payload.buildTask()
		if err != nil {
			apiWriteError(
				writer,
				http.StatusBadRequest,
				fmt.Errorf(
					"invalid compute task: %w",
					err,
				),
			)
			return
		}

		job, status, err := api.createFundedComputeJob(
			task,
			payload.Requester,
			payload.Reward,
			payload.Nonce,
		)
		if err != nil {
			apiWriteError(
				writer,
				status,
				err,
			)
			return
		}

		apiWriteJSON(
			writer,
			http.StatusCreated,
			map[string]any{
				"job": job,
			},
		)

	default:
		writer.Header().Set(
			"Allow",
			"GET, POST",
		)

		apiWriteError(
			writer,
			http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"),
		)
	}
}

func (api *apiServer) handleComputeJobAction(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if api.computeMarket == nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf("compute marketplace is unavailable"),
		)
		return
	}

	if request.Method != http.MethodPost {
		writer.Header().Set(
			"Allow",
			http.MethodPost,
		)

		apiWriteError(
			writer,
			http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"),
		)
		return
	}

	path := strings.TrimPrefix(
		request.URL.Path,
		"/api/v1/compute/jobs/",
	)

	path = strings.Trim(path, "/")

	parts := strings.Split(path, "/")

	if len(parts) != 2 ||
		parts[0] == "" ||
		parts[1] == "" {

		apiWriteError(
			writer,
			http.StatusNotFound,
			fmt.Errorf("invalid compute job endpoint"),
		)
		return
	}

	jobID := parts[0]
	action := parts[1]

	switch action {
	case "claim":
		handleComputeClaim(
			api,
			writer,
			request,
			jobID,
		)

	case "complete":
		handleComputeComplete(
			api,
			writer,
			request,
			jobID,
		)

	default:
		apiWriteError(
			writer,
			http.StatusNotFound,
			fmt.Errorf(
				"unknown compute job action: %s",
				action,
			),
		)
	}
}

func handleComputeClaim(
	api *apiServer,
	writer http.ResponseWriter,
	request *http.Request,
	jobID string,
) {
	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		4096,
	)

	var payload apiComputeClaimJobRequest

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid compute claim request: %w",
				err,
			),
		)
		return
	}

	job, err := api.computeMarket.Claim(
		jobID,
		payload.Worker,
	)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			err,
		)
		return
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"job": job,
		},
	)
}

func handleComputeComplete(
	api *apiServer,
	writer http.ResponseWriter,
	request *http.Request,
	jobID string,
) {
	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		65536,
	)

	var payload apiComputeCompleteJobRequest

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid compute completion request: %w",
				err,
			),
		)
		return
	}

	settlement, err := settleComputeJob(
		api,
		jobID,
		payload.Proof,
	)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			err,
		)
		return
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"verified":          true,
			"settled":           true,
			"job":               settlement.Job,
			"bountyReward":      settlement.BountyReward,
			"settlementTxId":    settlement.SettlementTxID,
			"block":             settlement.Block,
			"recovered":         settlement.Recovered,
			"crossChainReceipt": settlement.CrossChainReceipt,
		},
	)
}
