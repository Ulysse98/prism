package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"prism/internal/blockchain"
	"prism/internal/mempool"
	"prism/internal/storage"
	"prism/internal/transaction"
)

type apiFaucetRequest struct {
	Address string `json:"address"`
}

type apiFaucetResponse struct {
	Faucet           string `json:"faucet"`
	Recipient        string `json:"recipient"`
	Address          string `json:"address"`
	Amount           uint64 `json:"amount"`
	TxID             string `json:"txId"`
	Block            uint64 `json:"block"`
	TotalBalance     uint64 `json:"totalBalance"`
	AvailableBalance uint64 `json:"availableBalance"`
}

func (api *apiServer) handleFaucet(
	writer http.ResponseWriter,
	request *http.Request,
) {
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

	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		4096,
	)

	var payload apiFaucetRequest

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid faucet request: %w",
				err,
			),
		)
		return
	}

	payload.Address =
		strings.TrimSpace(payload.Address)

	if payload.Address == "" {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"faucet recipient cannot be empty",
			),
		)
		return
	}

	result, status, err :=
		api.dispenseFaucet(
			payload.Address,
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
		result,
	)
}

func (api *apiServer) dispenseFaucet(
	recipientIdentifier string,
) (
	apiFaucetResponse,
	int,
	error,
) {
	if !api.faucetEnabled {
		return apiFaucetResponse{},
			http.StatusForbidden,
			fmt.Errorf(
				"testnet faucet is disabled",
			)
	}

	if api.faucetAmount == 0 {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			fmt.Errorf(
				"testnet faucet amount is invalid",
			)
	}

	// Faucet block creation mutates the chain.
	// Serialize the complete read/validate/write cycle.
	api.stateMu.Lock()
	defer api.stateMu.Unlock()

	// Faucet signing requires a private local wallet.
	// Public API state alone is intentionally insufficient.
	if !storage.Exists(api.dataPath) {
		return apiFaucetResponse{},
			http.StatusServiceUnavailable,
			fmt.Errorf(
				"testnet faucet requires private local wallet state",
			)
	}

	chain, pos, wallets, err :=
		api.loadState()
	if err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			err
	}

	faucetName, faucetWallet, err :=
		resolveLocalWallet(
			api.faucetWallet,
			wallets,
		)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusServiceUnavailable,
			fmt.Errorf(
				"faucet wallet unavailable: %w",
				err,
			)
	}

	recipientAddress, recipientName, err :=
		resolveAddress(
			recipientIdentifier,
			wallets,
		)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusBadRequest,
			err
	}

	if faucetWallet.Address ==
		recipientAddress {

		return apiFaucetResponse{},
			http.StatusBadRequest,
			fmt.Errorf(
				"faucet cannot fund itself",
			)
	}

	if hasFaucetClaim(
		chain,
		faucetWallet.Address,
		recipientAddress,
		api.faucetAmount,
	) {
		return apiFaucetResponse{},
			http.StatusConflict,
			fmt.Errorf(
				"address has already claimed testnet faucet funds",
			)
	}

	available, err :=
		chain.AvailableBalanceOf(
			faucetWallet.Address,
		)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			err
	}

	if available < api.faucetAmount {
		return apiFaucetResponse{},
			http.StatusServiceUnavailable,
			fmt.Errorf(
				"faucet balance too low: available %d PRISM, required %d PRISM",
				available,
				api.faucetAmount,
			)
	}

	pool := mempool.New()

	nonce, err := pool.NextNonce(
		faucetWallet.Address,
		chain,
	)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			err
	}

	tx := transaction.New(
		faucetWallet.Address,
		recipientAddress,
		api.faucetAmount,
		nonce,
		faucetWallet.PublicKeyHex(),
	)

	if err := tx.Sign(
		faucetWallet.PrivateKey,
	); err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			fmt.Errorf(
				"unable to sign faucet transaction: %w",
				err,
			)
	}

	if err := pool.Add(
		tx,
		chain,
	); err != nil {
		return apiFaucetResponse{},
			http.StatusBadRequest,
			fmt.Errorf(
				"faucet transaction rejected: %w",
				err,
			)
	}

	if len(chain.Blocks) == 0 {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			fmt.Errorf(
				"blockchain is empty",
			)
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			err
	}

	block, err := chain.AddBlock(
		pool.Transactions(),
		nil,
		proposer.Address,
		pos,
	)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusBadRequest,
			fmt.Errorf(
				"unable to create faucet block: %w",
				err,
			)
	}

	if err := api.saveState(
		chain,
		pos,
		wallets,
	); err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			fmt.Errorf(
				"unable to persist faucet block: %w",
				err,
			)
	}

	totalBalance, err :=
		chain.BalanceOf(
			recipientAddress,
		)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			err
	}

	availableBalance, err :=
		chain.AvailableBalanceOf(
			recipientAddress,
		)
	if err != nil {
		return apiFaucetResponse{},
			http.StatusInternalServerError,
			err
	}

	return apiFaucetResponse{
			Faucet:           faucetName,
			Recipient:        recipientName,
			Address:          recipientAddress,
			Amount:           api.faucetAmount,
			TxID:             tx.ID,
			Block:            block.Height,
			TotalBalance:     totalBalance,
			AvailableBalance: availableBalance,
		},
		http.StatusCreated,
		nil
}

func hasFaucetClaim(
	chain *blockchain.Blockchain,
	faucetAddress string,
	recipientAddress string,
	amount uint64,
) bool {
	if chain == nil ||
		faucetAddress == "" ||
		recipientAddress == "" {

		return false
	}

	for _, block := range chain.Blocks {
		for _, tx := range block.Transactions {
			if tx.From == faucetAddress &&
				tx.To == recipientAddress &&
				tx.Amount == amount {

				return true
			}
		}
	}

	return false
}
