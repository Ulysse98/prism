package main

import (
	"fmt"
	"net/http"

	"prism/internal/consensus"
)

type apiReservedUsageResponse struct {
	Ecosystem uint64 `json:"ecosystem"`
	Treasury  uint64 `json:"treasury"`
	Team      uint64 `json:"team"`
	Liquidity uint64 `json:"liquidity"`
}

type apiReservedRemainingResponse struct {
	EcosystemRemaining uint64 `json:"ecosystemRemaining"`
	TreasuryRemaining  uint64 `json:"treasuryRemaining"`
	TeamRemaining      uint64 `json:"teamRemaining"`
	LiquidityRemaining uint64 `json:"liquidityRemaining"`
	TotalRemaining     uint64 `json:"totalRemaining"`
}

type apiReservedGrantResponse struct {
	Block           uint64 `json:"block"`
	ID              string `json:"id"`
	Pool            string `json:"pool"`
	Recipient       string `json:"recipient"`
	Amount          uint64 `json:"amount"`
	Nonce           uint64 `json:"nonce"`
	NotBeforeHeight uint64 `json:"notBeforeHeight"`
	ExpiresAtHeight uint64 `json:"expiresAtHeight"`
	Approvals       int    `json:"approvals"`
	Status          string `json:"status"`
}

type apiReservedRevocationResponse struct {
	Block     uint64 `json:"block"`
	ID        string `json:"id"`
	Pool      string `json:"pool"`
	GrantID   string `json:"grantId"`
	Approvals int    `json:"approvals"`
	Status    string `json:"status"`
}

type apiReservedResponse struct {
	Height        uint64                          `json:"height"`
	ExplicitUsed  uint64                          `json:"explicitUsed"`
	LegacyGenesis uint64                          `json:"legacyGenesis"`
	Usage         apiReservedUsageResponse        `json:"usage"`
	Remaining     apiReservedRemainingResponse    `json:"remaining"`
	Grants        []apiReservedGrantResponse      `json:"grants"`
	Revocations   []apiReservedRevocationResponse `json:"revocations"`
}

func (api *apiServer) handleReserved(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	chain, _, wallets, err := api.loadState()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
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

	accounting, err := chain.GetReservedAccountingState()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}

	explicitUsed, err := accounting.Usage.ExplicitTotal()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}

	remaining, err := accounting.Budget(
		consensus.DefaultSupplyPolicy(),
	)
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}

	height := chain.Blocks[len(chain.Blocks)-1].Height

	revoked := make(map[string]bool)

	revocations := make(
		[]apiReservedRevocationResponse,
		0,
	)

	for _, block := range chain.Blocks {
		for _, revocation := range block.ReservedRevocations {
			revoked[revocation.GrantID] = true

			revocations = append(
				revocations,
				apiReservedRevocationResponse{
					Block:     block.Height,
					ID:        revocation.ID,
					Pool:      string(revocation.Pool),
					GrantID:   revocation.GrantID,
					Approvals: len(revocation.Approvals),
					Status:    "REVOKED",
				},
			)
		}
	}

	grants := make(
		[]apiReservedGrantResponse,
		0,
	)

	for _, block := range chain.Blocks {
		for _, grant := range block.ReservedGrants {
			grants = append(
				grants,
				apiReservedGrantResponse{
					Block: block.Height,
					ID:    grant.ID,
					Pool:  string(grant.Pool),
					Recipient: walletNameForAddress(
						grant.Recipient,
						wallets,
					),
					Amount:          grant.Amount,
					Nonce:           grant.Nonce,
					NotBeforeHeight: grant.NotBeforeHeight,
					ExpiresAtHeight: grant.ExpiresAtHeight,
					Approvals:       len(grant.Approvals),
					Status: reservedGrantStatus(
						grant.NotBeforeHeight,
						grant.ExpiresAtHeight,
						height,
						revoked[grant.ID],
					),
				},
			)
		}
	}

	response := apiReservedResponse{
		Height:        height,
		ExplicitUsed:  explicitUsed,
		LegacyGenesis: accounting.Usage.LegacyGenesis,
		Usage: apiReservedUsageResponse{
			Ecosystem: accounting.Usage.Ecosystem,
			Treasury:  accounting.Usage.Treasury,
			Team:      accounting.Usage.Team,
			Liquidity: accounting.Usage.Liquidity,
		},
		Remaining: apiReservedRemainingResponse{
			EcosystemRemaining: remaining.EcosystemRemaining,
			TreasuryRemaining:  remaining.TreasuryRemaining,
			TeamRemaining:      remaining.TeamRemaining,
			LiquidityRemaining: remaining.LiquidityRemaining,
			TotalRemaining:     remaining.TotalRemaining,
		},
		Grants:      grants,
		Revocations: revocations,
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		response,
	)
}
