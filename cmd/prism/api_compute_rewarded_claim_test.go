package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestAPIComputeRejectsClaimForAlreadyRewardedTask(
	t *testing.T,
) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	bob := wallets["Bob"]
	if bob == nil {
		t.Fatal("Bob wallet missing")
	}

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{24, 27, 31},
	)
	if err != nil {
		t.Fatal(err)
	}

	proof, err := usefulwork.Execute(
		task,
		bob,
	)
	if err != nil {
		t.Fatal(err)
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := chain.AddBlock(
		nil,
		[]usefulwork.Proof{
			proof,
		},
		proposer.Address,
		pos,
	); err != nil {
		t.Fatalf(
			"failed to reward useful work task: %v",
			err,
		)
	}

	if !chain.HasRewardedUsefulWorkTask(
		task.ID,
	) {
		t.Fatal(
			"test setup did not reward useful work task",
		)
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

	job, err := market.Create(
		task,
		"Alice",
		25,
		47001,
	)
	if err != nil {
		t.Fatal(err)
	}

	genesisHash := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(
		genesisHash,
	)

	claim, err :=
		compute.SignClaimAuthorization(
			job.ID,
			chainID,
			genesisHash,
			bob,
		)
	if err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(
		claim,
	)
	if err != nil {
		t.Fatal(err)
	}

	api := &apiServer{
		dataPath:      dataPath,
		computeMarket: market,
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/compute/jobs/"+
			job.ID+
			"/claim",
		bytes.NewReader(body),
	)

	recorder := httptest.NewRecorder()

	api.handleComputeJobAction(
		recorder,
		request,
	)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected HTTP 409, got %d body=%s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	stored, err := market.Get(
		job.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != compute.JobStatusOpen {
		t.Fatalf(
			"rewarded task claim mutated job status: %s",
			stored.Status,
		)
	}

	if stored.Worker != "" {
		t.Fatalf(
			"rewarded task claim assigned worker: %s",
			stored.Worker,
		)
	}
}
