package main

import (
	"fmt"
	"net/http"
	"strings"

	"prism/internal/compute"
	"prism/internal/crosschain"
	"prism/internal/p2p"
	"prism/internal/usefulwork"
)

const (
	apiReceiptStatusVerified    = "verified"
	apiReceiptStatusPending     = "pending"
	apiReceiptStatusFailed      = "failed"
	apiReceiptStatusUnavailable = "unavailable"
)

type apiReceiptPrismStatus struct {
	Status            string `json:"status"`
	Block             uint64 `json:"block"`
	BlockHash         string `json:"blockHash"`
	ChainID           string `json:"chainId"`
	GenesisHash       string `json:"genesisHash"`
	ProofVersion      uint8  `json:"proofVersion"`
	SignatureVerified bool   `json:"signatureVerified"`
	ContextVerified   bool   `json:"contextVerified"`
}

type apiReceiptAnchor struct {
	Chain           string `json:"chain"`
	Status          string `json:"status"`
	TxHash          string `json:"txHash,omitempty"`
	RegistryAddress string `json:"registryAddress,omitempty"`
	BlockNumber     uint64 `json:"blockNumber,omitempty"`
	ExplorerURL     string `json:"explorerUrl,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
}

type apiReceiptResponse struct {
	JobID             string                `json:"jobId"`
	ProofID           string                `json:"proofId"`
	Task              string                `json:"task"`
	Worker            string                `json:"worker"`
	Reward            uint64                `json:"reward"`
	Prism             apiReceiptPrismStatus `json:"prism"`
	CrossChainReceipt crosschain.Receipt    `json:"crossChainReceipt"`
	Anchors           []apiReceiptAnchor    `json:"anchors"`
}

func apiReceiptAnchorStatus(
	settlement crosschain.Settlement,
) string {
	switch settlement.Status {
	case crosschain.SettlementStatusConfirmed:
		return apiReceiptStatusVerified

	case crosschain.SettlementStatusPending:
		return apiReceiptStatusPending

	case crosschain.SettlementStatusFailed:
		return apiReceiptStatusFailed

	default:
		return apiReceiptStatusUnavailable
	}
}

func apiReceiptAnchors(
	settlements []crosschain.Settlement,
) []apiReceiptAnchor {
	byChain := make(
		map[string]crosschain.Settlement,
		len(settlements),
	)

	for _, settlement := range settlements {
		byChain[settlement.Chain] = settlement
	}

	chains := []string{
		crosschain.SettlementChainArbitrum,
		crosschain.SettlementChainSolana,
	}

	anchors := make(
		[]apiReceiptAnchor,
		0,
		len(chains),
	)

	for _, chain := range chains {
		settlement, exists := byChain[chain]

		if !exists {
			anchors = append(
				anchors,
				apiReceiptAnchor{
					Chain:  chain,
					Status: apiReceiptStatusUnavailable,
				},
			)
			continue
		}

		anchors = append(
			anchors,
			apiReceiptAnchor{
				Chain:           chain,
				Status:          apiReceiptAnchorStatus(settlement),
				TxHash:          settlement.TxHash,
				RegistryAddress: settlement.RegistryAddress,
				BlockNumber:     settlement.BlockNumber,
				ExplorerURL:     settlement.ExplorerURL,
				UpdatedAt:       settlement.UpdatedAt,
			},
		)
	}

	return anchors
}

func (api *apiServer) handleReceiptByJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !apiGETOnly(writer, request) {
		return
	}

	if api.computeMarket == nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf(
				"compute marketplace is unavailable",
			),
		)
		return
	}

	jobID := strings.TrimPrefix(
		request.URL.Path,
		"/api/v1/receipts/",
	)

	jobID = strings.TrimSpace(jobID)

	if jobID == "" ||
		strings.Contains(jobID, "/") {

		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf(
				"invalid receipt job ID",
			),
		)
		return
	}

	job, err := api.computeMarket.Get(jobID)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusNotFound,
			err,
		)
		return
	}

	if job.Status != compute.JobStatusVerified ||
		job.ProofID == "" {

		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"compute job does not have a verified receipt yet",
			),
		)
		return
	}

	chain, _, _, err := api.loadState()
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	if len(chain.Blocks) == 0 ||
		chain.Blocks[0].Hash == "" {

		apiWriteError(
			writer,
			http.StatusInternalServerError,
			fmt.Errorf(
				"cannot verify receipt without a genesis block",
			),
		)
		return
	}

	genesisHash := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesisHash)

	var proof usefulwork.Proof
	var proofBlock uint64
	var proofBlockHash string
	proofFound := false

	for _, block := range chain.Blocks {
		for _, candidate := range block.UsefulWork {
			if candidate.ID != job.ProofID {
				continue
			}

			proof = candidate
			proofBlock = block.Height
			proofBlockHash = block.Hash
			proofFound = true
			break
		}

		if proofFound {
			break
		}
	}

	if !proofFound {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"verified compute proof is not present on the Prism chain",
			),
		)
		return
	}

	if proof.Worker != job.Worker {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"receipt proof worker does not match compute job",
			),
		)
		return
	}

	if proof.Task.ID != job.Task.ID {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"receipt proof task does not match compute job",
			),
		)
		return
	}

	if err := usefulwork.VerifyProof(proof); err != nil {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"invalid Prism proof: %w",
				err,
			),
		)
		return
	}

	if err := usefulwork.VerifyComputeProofContext(
		proof,
		job.ID,
		chainID,
		genesisHash,
	); err != nil {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf(
				"invalid Prism proof context: %w",
				err,
			),
		)
		return
	}

	receipt, err := crosschain.NewReceipt(
		job.ID,
		proof.ID,
		proof.Worker,
		chainID,
	)
	if err != nil {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			err,
		)
		return
	}

	var settlements []crosschain.Settlement

	if api.settlements != nil {
		settlements, err =
			api.settlements.ForRegistry(
				receipt.RegistryID,
			)

		if err != nil {
			apiWriteError(
				writer,
				http.StatusInternalServerError,
				err,
			)
			return
		}
	}

	response := apiReceiptResponse{
		JobID:   job.ID,
		ProofID: proof.ID,
		Task:    job.Task.Type,
		Worker:  proof.Worker,
		Reward:  job.Reward,
		Prism: apiReceiptPrismStatus{
			Status:            apiReceiptStatusVerified,
			Block:             proofBlock,
			BlockHash:         proofBlockHash,
			ChainID:           chainID,
			GenesisHash:       genesisHash,
			ProofVersion:      proof.ProofVersion,
			SignatureVerified: true,
			ContextVerified:   true,
		},
		CrossChainReceipt: receipt,
		Anchors: apiReceiptAnchors(
			settlements,
		),
	}

	apiWriteJSON(
		writer,
		http.StatusOK,
		response,
	)
}
