package compute

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	defer market.Close()

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

	if err := market.Close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewPersistentMarketplace(
		dataDir,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()

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

	dbPath := filepath.Join(
		dataDir,
		marketplaceDatabaseFilename,
	)

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf(
			"SQLite marketplace database missing: %v",
			err,
		)
	}
}

func TestPersistentMarketplaceMigratesLegacyJSON(
	t *testing.T,
) {
	dataDir := t.TempDir()

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{3, 4, 5},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := NewJob(
		task,
		"legacy-researcher",
		25,
		77,
	)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := marketplaceSnapshot{
		Version: 1,
		Jobs: []Job{
			job,
		},
	}

	data, err := json.MarshalIndent(
		snapshot,
		"",
		"  ",
	)
	if err != nil {
		t.Fatal(err)
	}

	legacyPath := filepath.Join(
		dataDir,
		marketplaceFilename,
	)

	if err := os.WriteFile(
		legacyPath,
		data,
		0600,
	); err != nil {
		t.Fatal(err)
	}

	market, err := NewPersistentMarketplace(
		dataDir,
	)
	if err != nil {
		t.Fatal(err)
	}

	stored, err := market.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Requester != job.Requester {
		t.Fatal(
			"legacy requester was not migrated",
		)
	}

	if stored.Reward != job.Reward {
		t.Fatal(
			"legacy reward was not migrated",
		)
	}

	if err := market.Close(); err != nil {
		t.Fatal(err)
	}

	// Prove SQLite is now authoritative.
	if err := os.Remove(legacyPath); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewPersistentMarketplace(
		dataDir,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()

	stored, err = reloaded.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.ID != job.ID {
		t.Fatal(
			"migrated SQLite job was not reloaded",
		)
	}
}
