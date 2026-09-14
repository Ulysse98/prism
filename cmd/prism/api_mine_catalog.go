package main

import (
	"fmt"
	"net/http"

	"prism/internal/usefulwork"
)

type apiMineCatalogEntry struct {
	ID         string `json:"id"`
	Task       string `json:"task"`
	Difficulty string `json:"difficulty"`
	WorkUnits  uint64 `json:"workUnits"`
	Reward     uint64 `json:"reward"`
}

type apiMineCatalogResponse struct {
	SourceChainHeight uint64                `json:"sourceChainHeight"`
	Tasks             []apiMineCatalogEntry `json:"tasks"`
}

func mineCatalogResponse(
	height uint64,
	usefulWorkEmission uint64,
) (apiMineCatalogResponse, error) {

	options, err :=
		mineTaskCatalogForHeight(height)
	if err != nil {
		return apiMineCatalogResponse{}, err
	}

	entries := make(
		[]apiMineCatalogEntry,
		0,
		len(options),
	)

	for _, option := range options {
		task := option.Task

		workUnits, err :=
			usefulwork.WorkUnits(task)
		if err != nil {
			return apiMineCatalogResponse{},
				fmt.Errorf(
					"cannot calculate work units for %s: %w",
					task.Type,
					err,
				)
		}

		reward, err :=
			mineRewardForWork(
				height+1,
				workUnits,
				usefulWorkEmission,
			)
		if err != nil {
			return apiMineCatalogResponse{},
				fmt.Errorf(
					"cannot calculate reward for %s: %w",
					task.Type,
					err,
				)
		}

		entries = append(
			entries,
			apiMineCatalogEntry{
				ID:         task.ID,
				Task:       task.Type,
				Difficulty: option.Difficulty,
				WorkUnits:  workUnits,
				Reward:     reward,
			},
		)
	}

	return apiMineCatalogResponse{
		SourceChainHeight: height,
		Tasks:             entries,
	}, nil
}

func (api *apiServer) handleMineTasks(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, _, _, err :=
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

	emission, err :=
		chain.GetEmissionState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf(
				"cannot calculate current emissions: %w",
				err,
			),
		)
		return
	}

	response, err :=
		mineCatalogResponse(
			lastBlock.Height,
			emission.UsefulWorkEmission,
		)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		response,
	)
}
