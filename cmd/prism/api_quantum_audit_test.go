package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

const quantumAuditTestToken = "7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c7c"

func quantumAuditHTTPFixture(
	t *testing.T,
) (
	*apiServer,
	compute.Job,
	usefulwork.Proof,
	[]*wallet.Wallet,
	*http.ServeMux,
) {
	t.Helper()
	t.Setenv("PRISM_QUANTUM_BACKEND", "")

	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}

	alice := wallets["Alice"]
	bob := wallets["Bob"]
	if alice == nil || bob == nil {
		t.Fatal("missing test wallets")
	}

	dataPath := t.TempDir()

	if err := storage.Save(
		dataPath, chain, pos, wallets,
	); err != nil {
		t.Fatal(err)
	}

	market, err := compute.NewPersistentMarketplace(dataPath)
	if err != nil {
		t.Fatal(err)
	}
	registerComputeMarketplaceCleanup(t, market)

	task, err := usefulwork.NewQuantumSimulationTask(2, 4096)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(task, "Alice", 31, 580201)
	if err != nil {
		t.Fatal(err)
	}

	job, err = market.Claim(job.ID, bob.Address)
	if err != nil {
		t.Fatal(err)
	}

	genesisHash := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesisHash)

	proof, err := usefulwork.ExecuteCompute(
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
	}

	settled, err := settleComputeJob(api, job.ID, proof)
	if err != nil {
		t.Fatal(err)
	}

	if settled.Job.Status != compute.JobStatusVerified {
		t.Fatal("quantum job was not settled")
	}

	job = settled.Job

	verifiers := make([]*wallet.Wallet, 3)
	addresses := make([]string, 3)

	for i := range verifiers {
		verifier, err := wallet.New()
		if err != nil {
			t.Fatal(err)
		}
		verifiers[i] = verifier
		addresses[i] = verifier.Address
	}

	api.quantumAuditPolicy = &usefulwork.QuantumQuorumPolicy{
		Model:               "bell-ideal-v1",
		AuthorizedVerifiers: addresses,
		RequiredApprovals:   2,
	}

	rawToken, err := hex.DecodeString(quantumAuditTestToken)
	if err != nil {
		t.Fatal(err)
	}

	tokenHash := sha256.Sum256(rawToken)
	api.quantumAuditTokenHash = &tokenHash
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/api/v1/compute/jobs/{id}/quantum-audit",
		api.handleQuantumAudit,
	)

	return api, job, proof, verifiers, mux
}

type quantumAuditHTTPState struct {
	Blocks       int
	Height       uint64
	LastHash     string
	TotalSupply  uint64
	AliceBalance uint64
	BobBalance   uint64
	Jobs         map[string]compute.Job
}

func snapshotQuantumAuditHTTP(
	t *testing.T,
	api *apiServer,
) quantumAuditHTTPState {
	t.Helper()

	chain, _, wallets, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	if len(chain.Blocks) == 0 {
		t.Fatal("empty blockchain")
	}

	supply, err := chain.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}

	alice, err := chain.BalanceOf(wallets["Alice"].Address)
	if err != nil {
		t.Fatal(err)
	}

	bob, err := chain.BalanceOf(wallets["Bob"].Address)
	if err != nil {
		t.Fatal(err)
	}

	last := chain.Blocks[len(chain.Blocks)-1]

	jobs := make(map[string]compute.Job)
	for _, job := range api.computeMarket.List() {
		jobs[job.ID] = job
	}

	return quantumAuditHTTPState{
		Blocks:       len(chain.Blocks),
		Height:       last.Height,
		LastHash:     last.Hash,
		TotalSupply:  supply,
		AliceBalance: alice,
		BobBalance:   bob,
		Jobs:         jobs,
	}
}

func quantumAuditHTTPBody(
	t *testing.T,
	reports []usefulwork.QuantumVerificationReport,
) []byte {
	t.Helper()

	data, err := json.Marshal(struct {
		Reports []usefulwork.QuantumVerificationReport `json:"reports"`
	}{
		Reports: reports,
	})

	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestQuantumAuditHTTPReadOnly(t *testing.T) {
	api, job, proof, verifiers, mux := quantumAuditHTTPFixture(t)

	first, err := usefulwork.SignQuantumVerificationReport(
		proof, verifiers[0],
	)
	if err != nil {
		t.Fatal(err)
	}

	second, err := usefulwork.SignQuantumVerificationReport(
		proof, verifiers[1],
	)
	if err != nil {
		t.Fatal(err)
	}

	third, err := usefulwork.SignQuantumVerificationReport(
		proof, verifiers[2],
	)
	if err != nil {
		t.Fatal(err)
	}

	outsider, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	strangerVote, err := usefulwork.SignQuantumVerificationReport(
		proof, outsider,
	)
	if err != nil {
		t.Fatal(err)
	}

	tampered := second
	tampered.Signature = "00"

	// Another quantum job exists, but has not been settled.
	pendingTask, err := usefulwork.NewQuantumSimulationTask(
		2, 8192,
	)
	if err != nil {
		t.Fatal(err)
	}

	pending, err := api.computeMarket.Create(
		pendingTask, "Alice", 19, 580202,
	)
	if err != nil {
		t.Fatal(err)
	}

	validURL := "/api/v1/compute/jobs/" +
		job.ID + "/quantum-audit"

	pendingURL := "/api/v1/compute/jobs/" +
		pending.ID + "/quantum-audit"

	missingURL := "/api/v1/compute/jobs/" +
		strings.Repeat("0", 64) + "/quantum-audit"

	twoVotes := quantumAuditHTTPBody(
		t,
		[]usefulwork.QuantumVerificationReport{
			first, second,
		},
	)

	threeVotes := quantumAuditHTTPBody(
		t,
		[]usefulwork.QuantumVerificationReport{
			first, second, third,
		},
	)

	tests := []struct {
		name          string
		method        string
		url           string
		body          []byte
		disablePolicy bool
		wantCode      int
		wantStatus    string
		wantQuorum    bool
	}{
		{
			name:       "TwoOfThreeProvisional",
			method:     http.MethodPost,
			url:        validURL,
			body:       twoVotes,
			wantCode:   200,
			wantStatus: usefulwork.QuantumAuditProvisional,
			wantQuorum: true,
		},
		{
			name:       "ThreeOfThreeComplete",
			method:     http.MethodPost,
			url:        validURL,
			body:       threeVotes,
			wantCode:   200,
			wantStatus: usefulwork.QuantumAuditComplete,
			wantQuorum: true,
		},
		{
			name:       "NoReports",
			method:     http.MethodPost,
			url:        validURL,
			body:       quantumAuditHTTPBody(t, nil),
			wantCode:   200,
			wantStatus: usefulwork.QuantumAuditNotAccepted,
		},
		{
			name:   "TamperedSignature",
			method: http.MethodPost,
			url:    validURL,
			body: quantumAuditHTTPBody(
				t,
				[]usefulwork.QuantumVerificationReport{
					first, tampered,
				},
			),
			wantCode:   200,
			wantStatus: usefulwork.QuantumAuditNotAccepted,
		},
		{
			name:   "UnauthorizedVerifier",
			method: http.MethodPost,
			url:    validURL,
			body: quantumAuditHTTPBody(
				t,
				[]usefulwork.QuantumVerificationReport{
					first, strangerVote,
				},
			),
			wantCode:   200,
			wantStatus: usefulwork.QuantumAuditNotAccepted,
		},
		{
			name:     "MalformedJSON",
			method:   http.MethodPost,
			url:      validURL,
			body:     []byte(`{"reports":`),
			wantCode: 400,
		},
		{
			name:     "RejectClientProvidedPolicy",
			method:   http.MethodPost,
			url:      validURL,
			body:     []byte(`{"reports":[],"policy":{}}`),
			wantCode: 400,
		},
		{
			name:     "OversizedBody",
			method:   http.MethodPost,
			url:      validURL,
			body:     []byte(strings.Repeat("x", 65537)),
			wantCode: 400,
		},
		{
			name:     "MethodNotAllowed",
			method:   http.MethodGet,
			url:      validURL,
			wantCode: 405,
		},
		{
			name:     "UnknownJob",
			method:   http.MethodPost,
			url:      missingURL,
			body:     twoVotes,
			wantCode: 404,
		},
		{
			name:     "UnsettledJob",
			method:   http.MethodPost,
			url:      pendingURL,
			body:     twoVotes,
			wantCode: 409,
		},
		{
			name:          "AuditDisabled",
			method:        http.MethodPost,
			url:           validURL,
			body:          twoVotes,
			disablePolicy: true,
			wantCode:      503,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotQuantumAuditHTTP(t, api)

			savedPolicy := api.quantumAuditPolicy
			if tc.disablePolicy {
				api.quantumAuditPolicy = nil
			}
			defer func() {
				api.quantumAuditPolicy = savedPolicy
			}()

			request := httptest.NewRequest(
				tc.method,
				tc.url,
				bytes.NewReader(tc.body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+quantumAuditTestToken)

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantCode {
				t.Fatalf(
					"HTTP status: got %d, want %d; body: %s",
					recorder.Code,
					tc.wantCode,
					recorder.Body.String(),
				)
			}

			if tc.wantStatus != "" {
				var response struct {
					Audit         usefulwork.QuantumAuditResult `json:"audit"`
					Observational bool                          `json:"observational"`
				}

				if err := json.Unmarshal(
					recorder.Body.Bytes(),
					&response,
				); err != nil {
					t.Fatal(err)
				}

				if !response.Observational ||
					response.Audit.Mode != "audit-only" ||
					response.Audit.Status != tc.wantStatus ||
					response.Audit.QuorumSatisfied != tc.wantQuorum ||
					response.Audit.JobID != job.ID ||
					response.Audit.ProofID != proof.ID ||
					response.Audit.PolicyFingerprint == "" {
					t.Fatalf(
						"unexpected audit response: %+v",
						response,
					)
				}
			}

			after := snapshotQuantumAuditHTTP(t, api)

			if !reflect.DeepEqual(before, after) {
				t.Fatalf(
					"HTTP audit changed Prism state:\nBEFORE: %+v\nAFTER: %+v",
					before,
					after,
				)
			}
		})
	}
}

func TestLoadQuantumAuditPolicy(t *testing.T) {
	_, _, _, verifiers, _ := quantumAuditHTTPFixture(t)

	valid := usefulwork.QuantumQuorumPolicy{
		Model: "bell-ideal-v1",
		AuthorizedVerifiers: []string{
			verifiers[0].Address,
			verifiers[1].Address,
			verifiers[2].Address,
		},
		RequiredApprovals: 2,
	}

	validData, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}

	duplicate := valid
	duplicate.AuthorizedVerifiers = []string{
		verifiers[0].Address,
		verifiers[0].Address,
		verifiers[2].Address,
	}

	duplicateData, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}

	impossible := valid
	impossible.RequiredApprovals = 4

	impossibleData, err := json.Marshal(impossible)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		content []byte
		valid   bool
	}{
		{"Valid", validData, true},
		{"DuplicateVerifier", duplicateData, false},
		{"ImpossibleQuorum", impossibleData, false},
		{"UnknownField", []byte(`{"Unknown":1}`), false},
		{"TrailingJSON", append(
			append([]byte(nil), validData...),
			[]byte(`{}`)...,
		), false},
		{"EmptyFile", nil, false},
		{"OversizedFile", []byte(strings.Repeat(" ", 16385)), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")

			if err := os.WriteFile(
				path, tc.content, 0600,
			); err != nil {
				t.Fatal(err)
			}

			loaded, err := loadQuantumAuditPolicy(path)

			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(loaded, valid) {
					t.Fatalf("policy changed during loading: %+v", loaded)
				}
			} else if err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}
