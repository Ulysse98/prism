package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"prism/internal/mempool"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

// Wallet transfers are experimental, opt-in, and intended for a local testnet.
// The current v1 transaction signature does not bind a chain ID. The chainId
// request check prevents accidental mistakes, NOT cross-chain replay attacks.
type apiSignedTransferRequest struct {
	ChainID     string                  `json:"chainId"`
	Transaction transaction.Transaction `json:"transaction"`
}

type apiSignedTransferResponse struct {
	TxID      string `json:"txId"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    uint64 `json:"amount"`
	Block     uint64 `json:"block"`
	BlockHash string `json:"blockHash"`
	ChainID   string `json:"chainId"`
	Status    string `json:"status"`
}

type apiWalletHistoryEntry struct {
	TxID      string    `json:"txId"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Amount    uint64    `json:"amount"`
	Nonce     uint64    `json:"nonce"`
	Block     uint64    `json:"block"`
	BlockHash string    `json:"blockHash"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

func (api *apiServer) handleWalletTransactions(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		api.handleWalletTransactionHistory(writer, request)
	case http.MethodPost:
		api.handleSignedWalletTransfer(writer, request)
	default:
		writer.Header().Set("Allow", "GET, POST")
		apiWriteError(writer, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (api *apiServer) handleSignedWalletTransfer(writer http.ResponseWriter, request *http.Request) {
	if !api.walletTransfersEnabled {
		apiWriteError(writer, http.StatusForbidden, fmt.Errorf("experimental wallet transfers are disabled"))
		return
	}
	if api.computeMarket == nil {
		apiWriteError(writer, http.StatusInternalServerError, fmt.Errorf("compute marketplace is unavailable"))
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, 8192)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload apiSignedTransferRequest
	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("invalid signed transfer request: %w", err))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("signed transfer request must contain exactly one JSON value"))
		return
	}

	// New wallet submissions require network-bound v2. Historical v1 blocks
	// remain readable, but v1 submissions are unsafe across networks.
	if payload.Transaction.Version != transaction.VersionV2 {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("wallet transfers require transaction version 2"))
		return
	}
	// Verify the client's Ed25519 signature. A private key is never received.
	if err := transaction.ValidateSigned(payload.Transaction); err != nil {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("invalid signed transaction: %w", err))
		return
	}
	if !isPrismAddress(payload.Transaction.To) {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("invalid recipient Prism address"))
		return
	}

	result, status, err := api.commitSignedWalletTransfer(payload)
	if err != nil {
		apiWriteError(writer, status, err)
		return
	}
	apiWriteJSON(writer, http.StatusCreated, result)
}

func (api *apiServer) commitSignedWalletTransfer(payload apiSignedTransferRequest) (apiSignedTransferResponse, int, error) {
	api.stateMu.Lock()
	defer api.stateMu.Unlock()

	chain, pos, wallets, err := api.loadState()
	if err != nil {
		return apiSignedTransferResponse{}, http.StatusInternalServerError, err
	}
	if len(chain.Blocks) == 0 {
		return apiSignedTransferResponse{}, http.StatusInternalServerError, fmt.Errorf("blockchain is empty")
	}
	// Canonical configured chain ID (not just a caller-supplied JSON hint).
	chainID, err := chain.ChainID()
	if err != nil {
		return apiSignedTransferResponse{}, http.StatusInternalServerError, err
	}
	if payload.ChainID != chainID || payload.Transaction.ChainID != chainID ||
		payload.Transaction.GenesisHash != chain.Blocks[0].Hash {
		return apiSignedTransferResponse{}, http.StatusBadRequest, fmt.Errorf("v2 transaction network context mismatch")
	}

	tx := payload.Transaction
	available, err := chain.AvailableBalanceOf(tx.From)
	if err != nil {
		return apiSignedTransferResponse{}, http.StatusInternalServerError, err
	}
	reserved, err := api.reservedWalletBounties(tx.From, wallets)
	if err != nil {
		return apiSignedTransferResponse{}, http.StatusInternalServerError, err
	}
	if available < reserved || tx.Amount > available-reserved {
		return apiSignedTransferResponse{}, http.StatusBadRequest,
			fmt.Errorf("insufficient unreserved wallet balance")
	}

	pool := mempool.New()
	if err := pool.Add(tx, chain); err != nil {
		return apiSignedTransferResponse{}, http.StatusConflict,
			fmt.Errorf("signed transaction rejected: %w", err)
	}
	tip := chain.Blocks[len(chain.Blocks)-1]
	proposer, err := pos.SelectProposer(tip.Hash, tip.Height+1)
	if err != nil {
		return apiSignedTransferResponse{}, http.StatusInternalServerError, err
	}
	block, err := chain.AddBlock(pool.Transactions(), nil, proposer.Address, pos)
	if err != nil {
		return apiSignedTransferResponse{}, http.StatusConflict,
			fmt.Errorf("unable to add signed transfer: %w", err)
	}
	if err := api.saveState(chain, pos, wallets); err != nil {
		return apiSignedTransferResponse{}, http.StatusInternalServerError,
			fmt.Errorf("unable to persist wallet transfer: %w", err)
	}
	return apiSignedTransferResponse{
		TxID: tx.ID, From: tx.From, To: tx.To, Amount: tx.Amount,
		Block: block.Height, BlockHash: block.Hash, ChainID: chainID, Status: "locally_included",
	}, http.StatusCreated, nil
}

// Keep funds reserved by OPEN/CLAIMED compute jobs unavailable for transfers.
// Historical job requesters may use either a local name or the wallet address.
func (api *apiServer) reservedWalletBounties(address string, wallets map[string]*wallet.Wallet) (uint64, error) {
	aliases := []string{address}
	for name, current := range wallets {
		if current != nil && current.Address == address {
			aliases = append(aliases, name)
		}
	}
	var total uint64
	seen := make(map[string]bool)
	for _, alias := range aliases {
		key := strings.ToLower(alias)
		if seen[key] {
			continue
		}
		seen[key] = true
		reserved, err := api.computeMarket.ReservedRewardFor(alias)
		if err != nil {
			return 0, err
		}
		if total > math.MaxUint64-reserved {
			return 0, fmt.Errorf("reserved compute reward overflow")
		}
		total += reserved
	}
	return total, nil
}

func (api *apiServer) handleWalletTransactionHistory(writer http.ResponseWriter, request *http.Request) {
	identifier := strings.TrimSpace(request.URL.Query().Get("address"))
	if identifier == "" {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("address query parameter is required"))
		return
	}
	limit := 25
	if raw := request.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("limit must be between 1 and 100"))
			return
		}
		limit = value
	}

	api.stateMu.Lock()
	defer api.stateMu.Unlock()
	chain, _, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}
	address, _, err := resolveAddress(identifier, wallets)
	if err != nil {
		apiWriteError(writer, http.StatusBadRequest, err)
		return
	}

	entries := make([]apiWalletHistoryEntry, 0)
	for blockIndex := len(chain.Blocks) - 1; blockIndex >= 0 && len(entries) < limit; blockIndex-- {
		block := chain.Blocks[blockIndex]
		for txIndex := len(block.Transactions) - 1; txIndex >= 0 && len(entries) < limit; txIndex-- {
			tx := block.Transactions[txIndex]
			if tx.From == "GENESIS" || (tx.From != address && tx.To != address) {
				continue
			}
			entries = append(entries, apiWalletHistoryEntry{
				TxID: tx.ID, From: tx.From, To: tx.To, Amount: tx.Amount,
				Nonce: tx.Nonce, Block: block.Height, BlockHash: block.Hash,
				Timestamp: block.Timestamp, Status: "locally_included",
			})
		}
	}
	apiWriteJSON(writer, http.StatusOK, map[string]any{
		"address": address, "count": len(entries), "entries": entries,
	})
}
