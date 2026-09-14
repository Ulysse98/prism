package compute

import (
	"testing"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func TestPersistentMarketplaceSurvivesReload(
	t *testing.T,
) {
	dataDir := t.TempDir()

	market, err := NewPersistentMarketplace(
		dataDir,
	)
	if err != nil {
		t.Fatal(err)
	}

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{8, 15, 16},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"colosseum-researcher",
		50,
		1001,
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := market.Claim(
		job.ID,
		worker.Address,
	); err != nil {
		t.Fatal(err)
	}

	proof, err := usefulwork.Execute(
		task,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := market.Complete(
		job.ID,
		proof,
	); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewPersistentMarketplace(
		dataDir,
	)
	if err != nil {
		t.Fatal(err)
	}

	stored, err := reloaded.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Status != JobStatusVerified {
		t.Fatalf(
			"expected VERIFIED after reload, got %s",
			stored.Status,
		)
	}

	if stored.Worker != worker.Address {
		t.Fatal(
			"worker was not persisted",
		)
	}

	if stored.ProofID != proof.ID {
		t.Fatal(
			"proof ID was not persisted",
		)
	}
}
