package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"prism/internal/participation"
)

func TestRewardClaimsRoundTrip(
	t *testing.T,
) {
	dataDir := t.TempDir()

	book := participation.NewRewardClaimBook()

	if err := book.MarkClaimed(
		"Bob",
		1,
	); err != nil {
		t.Fatal(err)
	}

	if err := book.MarkClaimed(
		"Alice",
		0,
	); err != nil {
		t.Fatal(err)
	}

	if err := SaveRewardClaims(
		dataDir,
		book,
	); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRewardClaims(
		dataDir,
	)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Count() != 2 {
		t.Fatalf(
			"expected 2 restored claims, got %d",
			loaded.Count(),
		)
	}

	if !loaded.IsClaimed("Alice", 0) {
		t.Fatal(
			"expected restored Alice claim",
		)
	}

	if !loaded.IsClaimed("Bob", 1) {
		t.Fatal(
			"expected restored Bob claim",
		)
	}
}

func TestLoadRewardClaimsMissingFile(
	t *testing.T,
) {
	book, err := LoadRewardClaims(
		t.TempDir(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if book.Count() != 0 {
		t.Fatalf(
			"expected empty claim book, got %d claims",
			book.Count(),
		)
	}
}

func TestLoadRewardClaimsRejectsDuplicate(
	t *testing.T,
) {
	dataDir := t.TempDir()

	state := RewardClaimsFile{
		Claims: []RewardClaimRecord{
			{
				Address: "Alice",
				Period:  0,
			},
			{
				Address: "Alice",
				Period:  0,
			},
		},
	}

	data, err := json.Marshal(
		state,
	)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(
		dataDir,
		rewardClaimsFilename,
	)

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadRewardClaims(
		dataDir,
	); err == nil {
		t.Fatal(
			"expected duplicate persisted claims to fail",
		)
	}
}
