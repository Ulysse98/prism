package compute

import (
	"testing"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func TestJobLifecycle(t *testing.T) {
	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{3, 4, 5},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := NewJob(
		task,
		"research-lab",
		25,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if job.Status != JobStatusOpen {
		t.Fatalf(
			"expected OPEN, got %s",
			job.Status,
		)
	}

	if job.WorkUnits != 3 {
		t.Fatalf(
			"expected 3 work units, got %d",
			job.WorkUnits,
		)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err := job.Claim(worker.Address); err != nil {
		t.Fatal(err)
	}

	if job.Status != JobStatusClaimed {
		t.Fatalf(
			"expected CLAIMED, got %s",
			job.Status,
		)
	}

	proof, err := usefulwork.Execute(
		task,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := job.Complete(proof); err != nil {
		t.Fatal(err)
	}

	if job.Status != JobStatusVerified {
		t.Fatalf(
			"expected VERIFIED, got %s",
			job.Status,
		)
	}

	if job.ProofID != proof.ID {
		t.Fatalf(
			"expected proof ID %s, got %s",
			proof.ID,
			job.ProofID,
		)
	}
}

func TestJobRejectsProofFromAnotherWorker(
	t *testing.T,
) {
	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{7, 11, 13},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := NewJob(
		task,
		"compute-requester",
		50,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}

	workerA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	workerB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if err := job.Claim(workerA.Address); err != nil {
		t.Fatal(err)
	}

	proof, err := usefulwork.Execute(
		task,
		workerB,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := job.Complete(proof); err == nil {
		t.Fatal(
			"expected foreign worker proof to be rejected",
		)
	}
}

func TestNewJobRejectsZeroReward(
	t *testing.T,
) {
	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{1, 2, 3},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewJob(
		task,
		"requester",
		0,
		3,
	)

	if err == nil {
		t.Fatal(
			"expected zero reward to be rejected",
		)
	}
}
