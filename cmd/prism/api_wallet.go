package main

import (
	"fmt"
	"net/http"
	"strings"
)

type apiWalletResponse struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	TotalBalance     uint64 `json:"totalBalance"`
	AvailableBalance uint64 `json:"availableBalance"`
	LockedStake      uint64 `json:"lockedStake"`
	Nonce            uint64 `json:"nonce"`
	HumanityVerified bool   `json:"humanityVerified"`
}

func (api *apiServer) handleWalletByAddress(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	identifier := strings.TrimPrefix(
		request.URL.Path,
		"/api/v1/wallets/",
	)

	if identifier == "" ||
		strings.Contains(identifier, "/") {

		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("wallet address or identifier is required"),
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

	address, name, err := resolveAddress(
		identifier,
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

	total, err := chain.BalanceOf(address)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	available, err := chain.AvailableBalanceOf(address)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	nonce, err := chain.NonceOf(address)
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
		apiWalletResponse{
			Name:             name,
			Address:          address,
			TotalBalance:     total,
			AvailableBalance: available,
			LockedStake:      chain.LockedStakeOf(address),
			Nonce:            nonce,
			HumanityVerified: chain.IsVerified(address),
		},
	)
}
