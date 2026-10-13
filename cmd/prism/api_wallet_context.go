package main

import (
	"net/http"

	"prism/internal/transaction"
)

// Returns the canonical configured chain identity used for v2 signing.
// This route is read-only and never reveals private wallet information.
func (api *apiServer) handleWalletSigningContext(writer http.ResponseWriter, request *http.Request) {
	if !apiGETOnly(writer, request) {
		return
	}
	chain, _, _, err := api.loadState()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}
	chainID, err := chain.ChainID()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}
	apiWriteJSON(writer, http.StatusOK, map[string]any{
		"chainId":            chainID,
		"genesisHash":        chain.Blocks[0].Hash,
		"transactionVersion": transaction.VersionV2,
	})
}
