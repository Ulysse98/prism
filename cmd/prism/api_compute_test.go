package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestAPIComputeCreateBuildsDotProductTask(t *testing.T) {
	payload := apiComputeCreateJobRequest{
		Type:    usefulwork.TaskTypeDotProduct,
		Values:  []uint64{10, 20, 30},
		ValuesB: []uint64{4, 5, 6},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if task.Type != usefulwork.TaskTypeDotProduct {
		t.Fatalf("unexpected task type: %s", task.Type)
	}

	if task.ID == "" || task.InputHash == "" {
		t.Fatal("expected canonical task ID and input hash")
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf("generated task is invalid: %v", err)
	}
}

func TestAPIComputeCreateBuildsMatrixTask(t *testing.T) {
	payload := apiComputeCreateJobRequest{
		Type:    usefulwork.TaskTypeMatrixMultiply,
		RowsA:   2,
		ColsA:   2,
		ColsB:   2,
		Values:  []uint64{1, 2, 3, 4},
		ValuesB: []uint64{5, 6, 7, 8},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf("generated matrix task is invalid: %v", err)
	}
}

func TestAPIComputeCreateBuildsConvolutionTask(t *testing.T) {
	payload := apiComputeCreateJobRequest{
		Type:       usefulwork.TaskTypeImageConvolution,
		Rows:       3,
		Cols:       3,
		KernelSize: 2,
		Values: []uint64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
		ValuesB: []uint64{
			1, 0,
			0, 1,
		},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf("generated convolution task is invalid: %v", err)
	}
}

func TestAPIComputeCreatePreservesLegacyTask(t *testing.T) {
	legacy, err := usefulwork.NewPrimeCountTask(
		[]uint64{2, 3, 4, 5, 11},
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := apiComputeCreateJobRequest{
		Task: &legacy,
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if task.ID != legacy.ID {
		t.Fatalf(
			"legacy task ID changed: got %s want %s",
			task.ID,
			legacy.ID,
		)
	}
}

func TestAPIComputeCreateRejectsMixedTaskFormats(t *testing.T) {
	legacy, err := usefulwork.NewSumSquaresTask(
		[]uint64{1, 2, 3},
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := apiComputeCreateJobRequest{
		Task:   &legacy,
		Type:   usefulwork.TaskTypeSumSquares,
		Values: []uint64{1, 2, 3},
	}

	_, err = payload.buildTask()
	if err == nil {
		t.Fatal("expected mixed task format to be rejected")
	}

	if !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIComputeFiltersMarketplaceJobs(
	t *testing.T,
) {
	market := compute.NewMarketplace()

	sumTask, err := usefulwork.NewSumSquaresTask(
		[]uint64{1, 2, 3},
	)
	if err != nil {
		t.Fatal(err)
	}

	primeTask, err := usefulwork.NewPrimeCountTask(
		[]uint64{2, 3, 4, 5},
	)
	if err != nil {
		t.Fatal(err)
	}

	openJob, err := market.Create(
		sumTask,
		"Alice",
		25,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	claimedJob, err := market.Create(
		sumTask,
		"Alice",
		50,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := market.Claim(
		claimedJob.ID,
		"prism_worker",
	); err != nil {
		t.Fatal(err)
	}

	if _, err := market.Create(
		primeTask,
		"Bob",
		100,
		3,
	); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/compute/jobs?status=OPEN&task=sum_squares&requester=Alice&minReward=20&limit=10",
		nil,
	)

	filters, err := parseComputeJobFilters(
		request,
	)
	if err != nil {
		t.Fatal(err)
	}

	jobs := filterComputeJobs(
		market.List(),
		filters,
	)

	if len(jobs) != 1 {
		t.Fatalf(
			"expected one filtered job, got %d",
			len(jobs),
		)
	}

	if jobs[0].ID != openJob.ID {
		t.Fatalf(
			"unexpected filtered job: %s",
			jobs[0].ID,
		)
	}
}

func TestAPIComputeRejectsInvalidFilters(
	t *testing.T,
) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/compute/jobs?status=INVALID",
		nil,
	)

	if _, err := parseComputeJobFilters(
		request,
	); err == nil {

		t.Fatal(
			"expected invalid status to be rejected",
		)
	}
}

func TestAPIComputeGetsJobByID(
	t *testing.T,
) {
	market := compute.NewMarketplace()

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{5, 6, 7},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"Alice",
		42,
		99,
	)
	if err != nil {
		t.Fatal(err)
	}

	api := &apiServer{
		computeMarket: market,
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/compute/jobs/"+job.ID,
		nil,
	)

	recorder := httptest.NewRecorder()

	api.handleComputeJobAction(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"unexpected HTTP status: %d",
			recorder.Code,
		)
	}

	var response struct {
		Job compute.Job `json:"job"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {

		t.Fatal(err)
	}

	if response.Job.ID != job.ID {
		t.Fatalf(
			"unexpected job ID: %s",
			response.Job.ID,
		)
	}
}

func TestAPIComputeAcceptsSignedClaim(
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

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{11, 13, 17},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"Alice",
		25,
		45001,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(chain.Blocks) == 0 {
		t.Fatal("test chain has no genesis block")
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

	body, err := json.Marshal(claim)
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

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"unexpected HTTP status: %d body=%s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Job compute.Job `json:"job"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if response.Job.Status !=
		compute.JobStatusClaimed {

		t.Fatalf(
			"unexpected job status: %s",
			response.Job.Status,
		)
	}

	if response.Job.Worker != bob.Address {
		t.Fatalf(
			"unexpected worker: %s",
			response.Job.Worker,
		)
	}
}

func TestAPIComputeRejectsForgedSignedClaim(
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

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{19, 23, 29},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"Alice",
		30,
		45002,
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

	// Attempt to claim the job as Alice using Bob's
	// signed authorization.
	claim.Worker = alice.Address

	body, err := json.Marshal(claim)
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

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected HTTP 401, got %d body=%s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	stored, err := market.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != compute.JobStatusOpen {
		t.Fatalf(
			"forged claim mutated job status: %s",
			stored.Status,
		)
	}

	if stored.Worker != "" {
		t.Fatalf(
			"forged claim assigned worker: %s",
			stored.Worker,
		)
	}
}
