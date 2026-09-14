package compute

import (
	"testing"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func TestMarketplaceLifecycle(t *testing.T) {
	market := NewMarketplace()

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{10, 20, 30},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"science-lab",
		100,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(market.List()) != 1 {
		t.Fatal(
			"expected one marketplace job",
		)
	}

	stored, err := market.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.ID != job.ID {
		t.Fatal(
			"stored job ID mismatch",
		)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claimed, err := market.Claim(
		job.ID,
		worker.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	if claimed.Status != JobStatusClaimed {
		t.Fatalf(
			"expected CLAIMED, got %s",
			claimed.Status,
		)
	}

	proof, err := usefulwork.Execute(
		task,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	completed, err := market.Complete(
		job.ID,
		proof,
	)
	if err != nil {
		t.Fatal(err)
	}

	if completed.Status != JobStatusVerified {
		t.Fatalf(
			"expected VERIFIED, got %s",
			completed.Status,
		)
	}

	if completed.ProofID != proof.ID {
		t.Fatal(
			"proof ID mismatch",
		)
	}
}

func TestMarketplaceRejectsDuplicateJob(
	t *testing.T,
) {
	market := NewMarketplace()

	task, err := usefulwork.NewPrimeCountTask(
		[]uint64{2, 3, 4, 5, 6, 7},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = market.Create(
		task,
		"researcher",
		20,
		42,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = market.Create(
		task,
		"researcher",
		20,
		42,
	)

	if err == nil {
		t.Fatal(
			"expected duplicate job rejection",
		)
	}
}

func TestMarketplaceRejectsDoubleClaim(
	t *testing.T,
) {
	market := NewMarketplace()

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{1, 4, 9},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"requester",
		10,
		7,
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

	if _, err := market.Claim(
		job.ID,
		workerA.Address,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := market.Claim(
		job.ID,
		workerB.Address,
	); err == nil {
		t.Fatal(
			"expected second claim to fail",
		)
	}
}

func TestMarketplaceUnknownJob(
	t *testing.T,
) {
	market := NewMarketplace()

	if _, err := market.Get(
		"does-not-exist",
	); err == nil {
		t.Fatal(
			"expected unknown job error",
		)
	}
}
