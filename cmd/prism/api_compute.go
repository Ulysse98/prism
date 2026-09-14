package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"prism/internal/usefulwork"
)

type apiComputeCreateJobRequest struct {
	Task      usefulwork.Task `json:"task"`
	Requester string          `json:"requester"`
	Reward    uint64          `json:"reward"`
	Nonce     uint64          `json:"nonce"`
}

type apiComputeClaimJobRequest struct {
	Worker string `json:"worker"`
}

type apiComputeCompleteJobRequest struct {
	Proof usefulwork.Proof `json:"proof"`
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

		job, err := api.computeMarket.Create(
			payload.Task,
			payload.Requester,
			payload.Reward,
			payload.Nonce,
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

	job, err := api.computeMarket.Complete(
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
			"verified": true,
			"job":      job,
		},
	)
}
