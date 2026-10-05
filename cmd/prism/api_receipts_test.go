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

	if len(decoded.Anchors) != 3 {
		t.Fatalf(
			"expected 3 anchors, got %d",
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

func TestReceiptAnchorsIncludesMonad(
	t *testing.T,
) {
	txHash :=
		"0x" +
			strings.Repeat(
				"de",
				32,
			)

	settlements :=
		[]crosschain.Settlement{
			{
				RegistryID: "0x" +
					strings.Repeat(
						"ef",
						32,
					),
				Chain: crosschain.
					SettlementChainMonad,
				Status: crosschain.
					SettlementStatusConfirmed,
				TxHash: txHash,
				RegistryAddress: "0x" +
					strings.Repeat(
						"34",
						20,
					),
				BlockNumber: 42,
				ExplorerURL: "https://testnet.monadscan.com/tx/" +
					txHash,
			},
		}

	anchors :=
		apiReceiptAnchors(
			settlements,
		)

	if len(anchors) != 3 {
		t.Fatalf(
			"expected 3 receipt anchors, got %d",
			len(anchors),
		)
	}

	monad :=
		anchors[2]

	if monad.Chain !=
		crosschain.SettlementChainMonad {

		t.Fatalf(
			"unexpected Monad chain: %s",
			monad.Chain,
		)
	}

	if monad.Status !=
		apiReceiptStatusVerified {

		t.Fatalf(
			"unexpected Monad status: %s",
			monad.Status,
		)
	}

	if monad.TxHash != txHash {
		t.Fatalf(
			"unexpected Monad tx hash: %s",
			monad.TxHash,
		)
	}
}
