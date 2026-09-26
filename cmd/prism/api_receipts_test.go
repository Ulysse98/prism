package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prism/internal/compute"
	"prism/internal/crosschain"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestReceiptAPIReconstructsVerifiedCrossChainReceipt(
	t *testing.T,
) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}

	alice := wallets["Alice"]
	if alice == nil {
		t.Fatal("Alice wallet missing")
	}

	bob := wallets["Bob"]
	if bob == nil {
		t.Fatal("Bob wallet missing")
	}

	if len(chain.Blocks) == 0 ||
		chain.Blocks[0].Hash == "" {

		t.Fatal("missing genesis block")
	}

	dataPath := t.TempDir()

	if err := storage.Save(
		dataPath,
		chain,
		pos,
		wallets,
	); err != nil {
		t.Fatal(err)
	}

	market, err :=
		compute.NewPersistentMarketplace(
			dataPath,
		)
	if err != nil {
		t.Fatal(err)
	}

	registerComputeMarketplaceCleanup(
		t,
		market,
	)

	settlements, err :=
		crosschain.NewSettlementStore(
			dataPath,
		)
	if err != nil {
		t.Fatal(err)
	}

	task, err :=
		usefulwork.NewSumSquaresTask(
			[]uint64{13, 21, 34},
		)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"Alice",
		25,
		440001,
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err = market.Claim(
		job.ID,
		bob.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	genesisHash :=
		chain.Blocks[0].Hash

	chainID :=
		p2p.MakeChainID(
			genesisHash,
		)

	proof, err :=
		usefulwork.ExecuteCompute(
			job.Task,
			job.ID,
			chainID,
			genesisHash,
			bob,
		)
	if err != nil {
		t.Fatal(err)
	}

	api := &apiServer{
		dataPath:      dataPath,
		computeMarket: market,
		settlements:   settlements,
	}

	result, err :=
		settleComputeJob(
			api,
			job.ID,
			proof,
		)
	if err != nil {
		t.Fatal(err)
	}

	if result.CrossChainReceipt == nil {
		t.Fatal(
			"missing cross-chain receipt",
		)
	}

	receipt :=
		result.CrossChainReceipt

	_, err =
		settlements.Upsert(
			crosschain.Settlement{
				RegistryID: receipt.RegistryID,
				Chain: crosschain.
					SettlementChainArbitrum,
				Status: crosschain.
					SettlementStatusConfirmed,
				TxHash: "0x" +
					strings.Repeat(
						"ab",
						32,
					),
				RegistryAddress: "0x" +
					strings.Repeat(
						"12",
						20,
					),
				BlockNumber: 440044,
			},
		)
	if err != nil {
		t.Fatal(err)
	}

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/receipts/"+job.ID,
			nil,
		)

	response :=
		httptest.NewRecorder()

	api.handleReceiptByJob(
		response,
		request,
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected HTTP 200, got %d: %s",
			response.Code,
			response.Body.String(),
		)
	}

	var decoded apiReceiptResponse

	if err :=
		json.NewDecoder(
			response.Body,
		).Decode(&decoded); err != nil {

		t.Fatal(err)
	}

	if decoded.JobID != job.ID {
		t.Fatalf(
			"job ID mismatch: got %s want %s",
			decoded.JobID,
			job.ID,
		)
	}

	if decoded.ProofID != proof.ID {
		t.Fatalf(
			"proof ID mismatch: got %s want %s",
			decoded.ProofID,
			proof.ID,
		)
	}

	if decoded.Prism.Status !=
		apiReceiptStatusVerified {

		t.Fatalf(
			"unexpected Prism status: %s",
			decoded.Prism.Status,
		)
	}

	if decoded.Prism.ProofVersion !=
		usefulwork.ComputeProofVersion {

		t.Fatalf(
			"unexpected proof version: %d",
			decoded.Prism.ProofVersion,
		)
	}

	if !decoded.Prism.SignatureVerified {
		t.Fatal(
			"expected verified signature",
		)
	}

	if !decoded.Prism.ContextVerified {
		t.Fatal(
			"expected verified proof context",
		)
	}

	if decoded.CrossChainReceipt.RegistryID !=
		receipt.RegistryID {

		t.Fatalf(
			"registry ID mismatch: got %s want %s",
			decoded.CrossChainReceipt.RegistryID,
			receipt.RegistryID,
		)
	}

	if len(decoded.Anchors) != 2 {
		t.Fatalf(
			"expected 2 anchors, got %d",
			len(decoded.Anchors),
		)
	}

	if decoded.Anchors[0].Chain !=
		crosschain.SettlementChainArbitrum {

		t.Fatalf(
			"unexpected first anchor chain: %s",
			decoded.Anchors[0].Chain,
		)
	}

	if decoded.Anchors[0].Status !=
		apiReceiptStatusVerified {

		t.Fatalf(
			"unexpected Arbitrum status: %s",
			decoded.Anchors[0].Status,
		)
	}

	if decoded.Anchors[1].Chain !=
		crosschain.SettlementChainSolana {

		t.Fatalf(
			"unexpected second anchor chain: %s",
			decoded.Anchors[1].Chain,
		)
	}

	if decoded.Anchors[1].Status !=
		apiReceiptStatusUnavailable {

		t.Fatalf(
			"unexpected Solana status: %s",
			decoded.Anchors[1].Status,
		)
	}
}
