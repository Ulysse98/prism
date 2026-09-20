package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"prism/internal/compute"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func newComputeFundingAPITest(
	t *testing.T,
) (*apiServer, string, uint64) {
	t.Helper()

	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}

	alice := wallets["Alice"]
	if alice == nil {
		t.Fatal("Alice wallet missing")
	}

	available, err :=
		chain.AvailableBalanceOf(
			alice.Address,
		)
	if err != nil {
		t.Fatal(err)
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

	return &apiServer{
			dataPath:      dataPath,
			computeMarket: market,
		},
		alice.Address,
		available
}

func computeFundingTask(
	t *testing.T,
	values []uint64,
) usefulwork.Task {
	t.Helper()

	task, err :=
		usefulwork.NewSumSquaresTask(
			values,
		)
	if err != nil {
		t.Fatal(err)
	}

	return task
}

func postComputeFundingJob(
	t *testing.T,
	api *apiServer,
	payload apiComputeCreateJobRequest,
) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/compute/jobs",
		bytes.NewReader(body),
	)

	recorder := httptest.NewRecorder()

	api.handleComputeJobs(
		recorder,
		request,
	)

	return recorder
}

func TestHandleComputeJobsAcceptsFullyFundedBounty(
	t *testing.T,
) {
	api, _, available :=
		newComputeFundingAPITest(t)

	if available == 0 {
		t.Fatal("Alice has no available balance")
	}

	task := computeFundingTask(
		t,
		[]uint64{2, 3, 5},
	)

	response := postComputeFundingJob(
		t,
		api,
		apiComputeCreateJobRequest{
			Task:      &task,
			Requester: "Alice",
			Reward:    available,
			Nonce:     1,
		},
	)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	if len(api.computeMarket.List()) != 1 {
		t.Fatal(
			"expected one funded compute job",
		)
	}
}

func TestHandleComputeJobsRejectsOvercommitAcrossRequesterAliases(
	t *testing.T,
) {
	api, aliceAddress, available :=
		newComputeFundingAPITest(t)

	if available < 3 {
		t.Fatalf(
			"insufficient test balance: %d",
			available,
		)
	}

	firstTask := computeFundingTask(
		t,
		[]uint64{7, 11, 13},
	)

	first := postComputeFundingJob(
		t,
		api,
		apiComputeCreateJobRequest{
			Task:      &firstTask,
			Requester: "alice",
			Reward:    available - 1,
			Nonce:     1,
		},
	)

	if first.Code != http.StatusCreated {
		t.Fatalf(
			"first job rejected: HTTP %d: %s",
			first.Code,
			first.Body.String(),
		)
	}

	secondTask := computeFundingTask(
		t,
		[]uint64{17, 19, 23},
	)

	second := postComputeFundingJob(
		t,
		api,
		apiComputeCreateJobRequest{
			Task:      &secondTask,
			Requester: aliceAddress,
			Reward:    2,
			Nonce:     2,
		},
	)

	if second.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			http.StatusBadRequest,
			second.Code,
			second.Body.String(),
		)
	}

	if !strings.Contains(
		second.Body.String(),
		"insufficient funded compute balance",
	) {
		t.Fatalf(
			"unexpected error response: %s",
			second.Body.String(),
		)
	}

	if len(api.computeMarket.List()) != 1 {
		t.Fatal(
			"rejected overcommit must not create a second job",
		)
	}
}
func TestHandleComputeJobsSerializesConcurrentFunding(
	t *testing.T,
) {
	api, _, available :=
		newComputeFundingAPITest(t)

	if available == 0 {
		t.Fatal("Alice has no available balance")
	}

	taskA := computeFundingTask(
		t,
		[]uint64{29, 31, 37},
	)

	taskB := computeFundingTask(
		t,
		[]uint64{41, 43, 47},
	)

	payloadA, err := json.Marshal(
		apiComputeCreateJobRequest{
			Task:      &taskA,
			Requester: "Alice",
			Reward:    available,
			Nonce:     101,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	payloadB, err := json.Marshal(
		apiComputeCreateJobRequest{
			Task:      &taskB,
			Requester: "alice",
			Reward:    available,
			Nonce:     102,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan int, 2)

	var wait sync.WaitGroup

	send := func(body []byte) {
		defer wait.Done()

		<-start

		request := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/compute/jobs",
			bytes.NewReader(body),
		)

		recorder := httptest.NewRecorder()

		api.handleComputeJobs(
			recorder,
			request,
		)

		results <- recorder.Code
	}

	wait.Add(2)

	go send(payloadA)
	go send(payloadB)

	close(start)

	wait.Wait()
	close(results)

	var created int
	var rejected int

	for status := range results {
		switch status {
		case http.StatusCreated:
			created++

		case http.StatusBadRequest:
			rejected++

		default:
			t.Fatalf(
				"unexpected concurrent HTTP status: %d",
				status,
			)
		}
	}

	if created != 1 {
		t.Fatalf(
			"expected exactly one funded job, got %d",
			created,
		)
	}

	if rejected != 1 {
		t.Fatalf(
			"expected exactly one rejected overcommit, got %d",
			rejected,
		)
	}

	if len(api.computeMarket.List()) != 1 {
		t.Fatalf(
			"expected one marketplace job after concurrent requests, got %d",
			len(api.computeMarket.List()),
		)
	}
}
