package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func computeSnapshotFixtureV064(t *testing.T) (compute.Job, *blockchain.Blockchain, usefulwork.Proof) {
	t.Helper()
	task, err := usefulwork.NewMLInferenceQuantizedTask(2, 3, 3,
		[]int64{1, 2, 3, 3, 2, 1},
		[]int64{1, 2, 0, 0, 1, 2, -1, 1, 1}, []int64{1, -1, 0})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	market := compute.NewMarketplace()
	job, err := market.Create(task, "Alice", 19, 4211)
	if err != nil {
		t.Fatal(err)
	}
	job, err = market.Claim(job.ID, worker.Address)
	if err != nil {
		t.Fatal(err)
	}
	genesis := "genesis-v064-phase2-fixture"
	proof, err := usefulwork.ExecuteCompute(task, job.ID, p2p.MakeChainID(genesis), genesis, worker)
	if err != nil {
		t.Fatal(err)
	}
	job, err = market.Complete(job.ID, proof)
	if err != nil {
		t.Fatal(err)
	}
	chain := &blockchain.Blockchain{Blocks: []blockchain.Block{
		{Height: 0, Hash: genesis},
		{Height: 1, Hash: "example-block-hash", UsefulWork: []usefulwork.Proof{proof}},
	}}
	return job, chain, proof
}

func TestComputeProofSnapshotV064Valid(t *testing.T) {
	job, chain, proof := computeSnapshotFixtureV064(t)
	snapshot, err := computeProofSnapshotV064(job, chain)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Proof.ID != proof.ID || snapshot.Job.ID != job.ID || snapshot.Block != 1 {
		t.Fatal("snapshot does not match verified job and on-chain proof")
	}
}

func TestComputeProofSnapshotV064RejectsMismatchAndReplay(t *testing.T) {
	job, chain, _ := computeSnapshotFixtureV064(t)
	cases := []struct {
		name   string
		mutate func(*compute.Job, *blockchain.Blockchain)
	}{
		{"wrong_job", func(j *compute.Job, _ *blockchain.Blockchain) { j.Nonce++ }},
		{"wrong_worker", func(j *compute.Job, _ *blockchain.Blockchain) { j.Worker = "forged" }},
		{"wrong_proof", func(j *compute.Job, _ *blockchain.Blockchain) { j.ProofID = "forged" }},
		{"wrong_genesis", func(_ *compute.Job, c *blockchain.Blockchain) { c.Blocks[0].Hash = "another-genesis" }},
		{"proof_tamper", func(_ *compute.Job, c *blockchain.Blockchain) { c.Blocks[1].UsefulWork[0].ResultValues[0] = 999 }},
		{"missing_proof", func(_ *compute.Job, c *blockchain.Blockchain) { c.Blocks[1].UsefulWork = nil }},
		{"duplicate_proof", func(_ *compute.Job, c *blockchain.Blockchain) {
			c.Blocks[1].UsefulWork = append(c.Blocks[1].UsefulWork, c.Blocks[1].UsefulWork[0])
		}},
		{"wrong_work_units", func(j *compute.Job, _ *blockchain.Blockchain) { j.WorkUnits++ }},
		{"not_verified", func(j *compute.Job, _ *blockchain.Blockchain) { j.Status = compute.JobStatusClaimed }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// JSON clone prevents aliasing slices across cases.
			jc := job
			var cc blockchain.Blockchain
			encoded, err := json.Marshal(chain)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &cc); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&jc, &cc)
			if _, err := computeProofSnapshotV064(jc, &cc); err == nil {
				t.Fatal("invalid verified proof snapshot was accepted")
			}
		})
	}
}

func TestComputeProofEndpointV064RejectsPendingAndWrites(t *testing.T) {
	task, err := usefulwork.NewMLInferenceQuantizedTask(1, 3, 3,
		[]int64{1, 2, 3}, []int64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []int64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	market := compute.NewMarketplace()
	job, err := market.Create(task, "Alice", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	api := &apiServer{computeMarket: market}
	endpoint := "/api/v1/compute/proofs/" + job.ID
	for _, tc := range []struct {
		method string
		code   int
	}{
		{http.MethodGet, http.StatusConflict},
		{http.MethodPost, http.StatusMethodNotAllowed},
	} {
		recorder := httptest.NewRecorder()
		api.handleComputeProofV064(recorder, httptest.NewRequest(tc.method, endpoint, nil))
		if recorder.Code != tc.code {
			t.Fatalf("%s: got %d want %d", tc.method, recorder.Code, tc.code)
		}
	}
	if got, err := market.Get(job.ID); err != nil || got.Status != compute.JobStatusOpen {
		t.Fatal("read-only proof endpoint mutated marketplace")
	}
}

func TestComputeProofEndpointV064SettledReadOnlyE2E(t *testing.T) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	dataPath := t.TempDir()
	if err := storage.Save(dataPath, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}
	market, err := compute.NewPersistentMarketplace(dataPath)
	if err != nil {
		t.Fatal(err)
	}
	registerComputeMarketplaceCleanup(t, market)
	task, err := usefulwork.NewMLInferenceQuantizedTask(2, 3, 3,
		[]int64{1, 2, 3, 3, 2, 1}, []int64{1, 2, 0, 0, 1, 2, -1, 1, 1}, []int64{1, -1, 0})
	if err != nil {
		t.Fatal(err)
	}
	job, err := market.Create(task, "Alice", 11, 65929)
	if err != nil {
		t.Fatal(err)
	}
	job, err = market.Claim(job.ID, wallets["Bob"].Address)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, job.ID,
		p2p.MakeChainID(chain.Blocks[0].Hash), chain.Blocks[0].Hash, wallets["Bob"])
	if err != nil {
		t.Fatal(err)
	}
	api := &apiServer{dataPath: dataPath, computeMarket: market}
	if _, err := settleComputeJob(api, job.ID, proof); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	api.handleComputeProofV064(recorder, httptest.NewRequest(http.MethodGet,
		"/api/v1/compute/proofs/"+job.ID, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("proof GET: HTTP %d body=%s", recorder.Code, recorder.Body.String())
	}
	var snapshot apiComputeProofSnapshotV064
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Job.ID != job.ID || snapshot.Proof.ID != proof.ID || snapshot.Block == 0 {
		t.Fatal("wrong settled proof snapshot")
	}
	if err := usefulwork.VerifyProof(snapshot.Proof); err != nil {
		t.Fatal(err)
	}
	verifier, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	report, err := usefulwork.SignComputeVerificationV064(snapshot.Proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := usefulwork.VerifyComputeVerificationV064(report, snapshot.Proof); err != nil {
		t.Fatal(err)
	}
}
