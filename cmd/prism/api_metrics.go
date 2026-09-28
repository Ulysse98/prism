package main

import (
	"fmt"
	"io"
	"net/http"
)

type apiMetricsSnapshot struct {
	Height      uint64
	Blocks      int
	Validators  int
	TotalStake  uint64
	TotalSupply uint64
	ChainValid  bool
}

func (api *apiServer) handleMetrics(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, pos, _, err := api.loadState()
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

	totalSupply, err := chain.TotalSupply()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	last := chain.Blocks[len(chain.Blocks)-1]

	snapshot := apiMetricsSnapshot{
		Height:      last.Height,
		Blocks:      len(chain.Blocks),
		Validators:  len(pos.Validators),
		TotalStake:  pos.TotalStake(),
		TotalSupply: totalSupply,
		ChainValid:  chain.ValidateChain(pos),
	}

	writer.Header().Set(
		"Content-Type",
		"text/plain; version=0.0.4; charset=utf-8",
	)

	writer.WriteHeader(http.StatusOK)

	writeAPIMetrics(
		writer,
		snapshot,
	)
}

func writeAPIMetrics(
	writer io.Writer,
	snapshot apiMetricsSnapshot,
) {
	chainValid := 0
	if snapshot.ChainValid {
		chainValid = 1
	}

	fmt.Fprintf(
		writer,
		"# HELP prism_chain_height Current Prism blockchain height.\n"+
			"# TYPE prism_chain_height gauge\n"+
			"prism_chain_height %d\n"+
			"# HELP prism_chain_blocks Number of blocks currently stored by the Prism node.\n"+
			"# TYPE prism_chain_blocks gauge\n"+
			"prism_chain_blocks %d\n"+
			"# HELP prism_validators Number of active Prism validators.\n"+
			"# TYPE prism_validators gauge\n"+
			"prism_validators %d\n"+
			"# HELP prism_total_stake Total PRISM currently staked.\n"+
			"# TYPE prism_total_stake gauge\n"+
			"prism_total_stake %d\n"+
			"# HELP prism_total_supply Current PRISM token supply.\n"+
			"# TYPE prism_total_supply gauge\n"+
			"prism_total_supply %d\n"+
			"# HELP prism_chain_valid Whether the local Prism chain validates successfully.\n"+
			"# TYPE prism_chain_valid gauge\n"+
			"prism_chain_valid %d\n",
		snapshot.Height,
		snapshot.Blocks,
		snapshot.Validators,
		snapshot.TotalStake,
		snapshot.TotalSupply,
		chainValid,
	)
}
