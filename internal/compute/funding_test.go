package compute

import (
	"math"
	"testing"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func createFundingTestJob(
	t *testing.T,
	market *Marketplace,
	requester string,
	reward uint64,
	nonce uint64,
) Job {
	t.Helper()

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{1, 2, 3},
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		requester,
		reward,
		nonce,
	)
	if err != nil {
		t.Fatal(err)
	}

	return job
}

func TestReservedRewardForEmptyMarketplace(
	t *testing.T,
) {
	market := NewMarketplace()

	reserved, err := market.ReservedRewardFor(
		"Alice",
	)
	if err != nil {
		t.Fatal(err)
	}

	if reserved != 0 {
		t.Fatalf(
			"expected 0 reserved PRISM, got %d",
			reserved,
		)
	}
}

func TestReservedRewardForCountsOpenJobs(
	t *testing.T,
) {
	market := NewMarketplace()

	createFundingTestJob(
		t,
		market,
		"Alice",
		20,
		1,
	)

	createFundingTestJob(
		t,
		market,
		"Alice",
		30,
		2,
	)

	reserved, err := market.ReservedRewardFor(
		"Alice",
	)
	if err != nil {
		t.Fatal(err)
	}

	if reserved != 50 {
		t.Fatalf(
			"expected 50 reserved PRISM, got %d",
			reserved,
		)
	}
}

func TestReservedRewardForCountsClaimedJobs(
	t *testing.T,
) {
	market := NewMarketplace()

	createFundingTestJob(
		t,
		market,
		"Alice",
		20,
		1,
	)

	job := createFundingTestJob(
		t,
		market,
		"Alice",
		30,
		2,
	)

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

	reserved, err := market.ReservedRewardFor(
		"Alice",
	)
	if err != nil {
		t.Fatal(err)
	}

	if reserved != 50 {
		t.Fatalf(
			"expected 50 reserved PRISM, got %d",
			reserved,
		)
	}
}

func TestReservedRewardForIgnoresVerifiedJobs(
	t *testing.T,
) {
	market := NewMarketplace()

	job := createFundingTestJob(
		t,
		market,
		"Alice",
		40,
		1,
	)

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
		job.Task,
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

	reserved, err := market.ReservedRewardFor(
		"Alice",
	)
	if err != nil {
		t.Fatal(err)
	}

	if reserved != 0 {
		t.Fatalf(
			"expected verified bounty to be released, got %d",
			reserved,
		)
	}
}

func TestReservedRewardForIgnoresOtherRequesters(
	t *testing.T,
) {
	market := NewMarketplace()

	createFundingTestJob(
		t,
		market,
		"Alice",
		20,
		1,
	)

	createFundingTestJob(
		t,
		market,
		"Bob",
		70,
		2,
	)

	reserved, err := market.ReservedRewardFor(
		"Alice",
	)
	if err != nil {
		t.Fatal(err)
	}

	if reserved != 20 {
		t.Fatalf(
			"expected 20 reserved PRISM, got %d",
			reserved,
		)
	}
}

func TestReservedRewardForRejectsOverflow(
	t *testing.T,
) {
	market := NewMarketplace()

	createFundingTestJob(
		t,
		market,
		"Alice",
		math.MaxUint64,
		1,
	)

	createFundingTestJob(
		t,
		market,
		"Alice",
		1,
		2,
	)

	if _, err := market.ReservedRewardFor(
		"Alice",
	); err == nil {
		t.Fatal(
			"expected reserved reward overflow",
		)
	}
}

func TestReservedRewardForRejectsEmptyRequester(
	t *testing.T,
) {
	market := NewMarketplace()

	if _, err := market.ReservedRewardFor(
		"   ",
	); err == nil {
		t.Fatal(
			"expected empty requester rejection",
		)
	}
}

func TestReservedRewardForMatchesRequesterCaseInsensitively(
	t *testing.T,
) {
	market := NewMarketplace()

	createFundingTestJob(
		t,
		market,
		"alice",
		25,
		1,
	)

	reserved, err := market.ReservedRewardFor(
		"Alice",
	)
	if err != nil {
		t.Fatal(err)
	}

	if reserved != 25 {
		t.Fatalf(
			"expected 25 reserved PRISM, got %d",
			reserved,
		)
	}
}
