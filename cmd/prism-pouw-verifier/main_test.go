package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func verifierFixtureV064(t *testing.T) (compute.Job, proofSnapshotV064, *blockchain.Blockchain, *wallet.Wallet, *wallet.Wallet) {
	t.Helper()
	task, err := usefulwork.NewMLInferenceQuantizedTask(2, 3, 3,
		[]int64{1, 2, 3, 3, 2, 1}, []int64{1, 2, 0, 0, 1, 2, -1, 1, 1}, []int64{1, -1, 0})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	market := compute.NewMarketplace()
	job, err := market.Create(task, "Alice", 19, 42)
	if err != nil {
		t.Fatal(err)
	}
	job, err = market.Claim(job.ID, worker.Address)
	if err != nil {
		t.Fatal(err)
	}
	genesis := "isolated-verifier-genesis"
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
		{Height: 1, Hash: "isolated-block-1", UsefulWork: []usefulwork.Proof{proof}},
	}}
	snap := proofSnapshotV064{
		Job: job, Proof: proof, Block: 1, BlockHash: "isolated-block-1",
		ChainID: p2p.MakeChainID(genesis), GenesisHash: genesis,
	}
	return job, snap, chain, verifier, worker
}

func cloneSnapshotV064(t *testing.T, original proofSnapshotV064) proofSnapshotV064 {
	t.Helper()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var cloned proofSnapshotV064
	if err := json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func TestVerifierV064LocalAPIOnly(t *testing.T) {
	for _, base := range []string{
		"https://127.0.0.1:8080/api/v1", "http://example.org:8080/api/v1",
		"http://user:pass@127.0.0.1:8080/api/v1", "http://127.0.0.1/api/v1",
		"http://127.0.0.1:8080/api/v1?x=1", "http://127.0.0.1:8080/api/v1/other",
	} {
		if _, err := validateLocalAPIBaseV064(base); err == nil {
			t.Fatalf("accepted unsafe API %q", base)
		}
	}
	if _, err := validateLocalAPIBaseV064("http://127.0.0.1:8080/api/v1"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierV064SnapshotAndAdversarialCases(t *testing.T) {
	job, snap, chain, _, _ := verifierFixtureV064(t)
	if err := validateProofSnapshotV064(job, snap, chain); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*compute.Job, *proofSnapshotV064, *blockchain.Blockchain)
	}{
		{"wrong_network", func(_ *compute.Job, s *proofSnapshotV064, _ *blockchain.Blockchain) { s.ChainID = "bad" }},
		{"wrong_genesis", func(_ *compute.Job, s *proofSnapshotV064, _ *blockchain.Blockchain) { s.GenesisHash = "bad" }},
		{"wrong_block_hash", func(_ *compute.Job, s *proofSnapshotV064, _ *blockchain.Blockchain) { s.BlockHash = "bad" }},
		{"wrong_job", func(j *compute.Job, _ *proofSnapshotV064, _ *blockchain.Blockchain) { j.Nonce++ }},
		{"wrong_worker", func(_ *compute.Job, s *proofSnapshotV064, _ *blockchain.Blockchain) { s.Proof.Worker = "forged" }},
		{"corrupt_result", func(_ *compute.Job, s *proofSnapshotV064, _ *blockchain.Blockchain) { s.Proof.ResultValues[0] = 999 }},
		{"missing_chain_proof", func(_ *compute.Job, _ *proofSnapshotV064, c *blockchain.Blockchain) { c.Blocks[1].UsefulWork = nil }},
		{"duplicate_chain_proof", func(_ *compute.Job, _ *proofSnapshotV064, c *blockchain.Blockchain) {
			c.Blocks[1].UsefulWork = append(c.Blocks[1].UsefulWork, c.Blocks[1].UsefulWork[0])
		}},
		{"wrong_units", func(_ *compute.Job, s *proofSnapshotV064, _ *blockchain.Blockchain) { s.Job.WorkUnits++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clone all nested slices to make every adversarial test independent.
			originalBytes, err := json.Marshal(chain)
			if err != nil {
				t.Fatal(err)
			}
			var c blockchain.Blockchain
			if err := json.Unmarshal(originalBytes, &c); err != nil {
				t.Fatal(err)
			}
			s := cloneSnapshotV064(t, snap)
			j := job
			tc.mutate(&j, &s, &c)
			if err := validateProofSnapshotV064(j, s, &c); err == nil {
				t.Fatal("accepted malicious snapshot")
			}
		})
	}
}

func TestVerifierV064CollectReportsHTTP(t *testing.T) {
	job, snap, chain, verifier, worker := verifierFixtureV064(t)
	var corrupt, wrongStatus, duplicate atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/status":
			_ = json.NewEncoder(w).Encode(nodeStatusV064{ChainValid: !wrongStatus.Load(), ChainID: snap.ChainID, GenesisHash: snap.GenesisHash})
		case "/api/v1/compute/jobs":
			if r.URL.Query().Get("status") != "VERIFIED" || r.URL.Query().Get("task") != usefulwork.TaskTypeMLInferenceQuantized {
				t.Error("wrong discovery query")
			}
			jobs := []compute.Job{job}
			if duplicate.Load() {
				jobs = append(jobs, job)
			}
			_ = json.NewEncoder(w).Encode(jobListV064{Count: len(jobs), Jobs: jobs})
		case "/api/v1/compute/proofs/" + job.ID:
			candidate := snap
			candidate.Proof.ResultValues = append([]uint64(nil), snap.Proof.ResultValues...)
			if corrupt.Load() {
				candidate.Proof.ResultValues[0] = 999
			}
			_ = json.NewEncoder(w).Encode(candidate)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := server.Client()
	base := server.URL + "/api/v1"
	reports, err := collectReportsV064(context.Background(), client, base, chain, verifier, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("got %d reports", len(reports))
	}
	if reports[0].JobID != job.ID {
		t.Fatal("report job mismatch")
	}
	if err := usefulwork.VerifyComputeVerificationV064(reports[0], snap.Proof); err != nil {
		t.Fatal(err)
	}
	corrupt.Store(true)
	if result, err := collectReportsV064(context.Background(), client, base, chain, verifier, 100); err == nil || len(result) != 0 {
		t.Fatal("corrupt result emitted report")
	}
	corrupt.Store(false)
	duplicate.Store(true)
	if result, err := collectReportsV064(context.Background(), client, base, chain, verifier, 100); err == nil || len(result) != 0 {
		t.Fatal("duplicate listing emitted report")
	}
	duplicate.Store(false)
	wrongStatus.Store(true)
	if result, err := collectReportsV064(context.Background(), client, base, chain, verifier, 100); err == nil || len(result) != 0 {
		t.Fatal("invalid chain status emitted report")
	}
	wrongStatus.Store(false)
	if result, err := collectReportsV064(context.Background(), client, base, chain, worker, 100); err == nil || len(result) != 0 {
		t.Fatal("worker self-verification emitted report")
	}
}

func TestVerifierV064HTTPRejectsRedirectAndMalformedData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusTemporaryRedirect)
		case "/bad":
			_, _ = w.Write([]byte(`{"count":"bad","jobs":[]}`))
		case "/huge":
			_, _ = w.Write([]byte(strings.Repeat(" ", maxHTTPBodyV064+1)))
		case "/ok":
			_, _ = w.Write([]byte(`{"count":0,"jobs":[]}`))
		}
	}))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	var got jobListV064
	for _, path := range []string{"/redirect", "/bad", "/huge"} {
		if err := readJSONV064(context.Background(), client, server.URL+path, &got); err == nil {
			t.Fatalf("accepted bad HTTP response at %s", path)
		}
	}
	got = jobListV064{}
	if err := readJSONV064(context.Background(), client, server.URL+"/ok", &got); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	if got.Count != 0 || got.Jobs == nil || len(got.Jobs) != 0 {
		t.Fatalf("unexpected empty listing: count=%d jobs=%#v", got.Count, got.Jobs)
	}
}
