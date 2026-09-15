package main

import (
	"testing"

	"prism/internal/compute"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestSettleComputeJobRetryDoesNotDoublePay(
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

	task, err :=
		usefulwork.NewSumSquaresTask(
			[]uint64{21, 34, 55},
		)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"Alice",
		25,
		123456,
	)
	if err != nil {
		t.Fatal(err)
	}

	job, err = market.Claim(
		job.ID,
		bob.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	proof, err :=
		usefulwork.Execute(
			job.Task,
			bob,
		)
	if err != nil {
		t.Fatal(err)
	}

	api := &apiServer{
		dataPath:      dataPath,
		computeMarket: market,
	}

	first, err :=
		settleComputeJob(
			api,
			job.ID,
			proof,
		)
	if err != nil {
		t.Fatal(err)
	}

	if first.Recovered {
		t.Fatal(
			"first settlement must not be recovery",
		)
	}

	if first.Job.Status !=
		compute.JobStatusVerified {

		t.Fatalf(
			"unexpected first status: %s",
			first.Job.Status,
		)
	}

	if first.Job.ProofID != proof.ID {
		t.Fatalf(
			"unexpected proof ID: got %s want %s",
			first.Job.ProofID,
			proof.ID,
		)
	}

	if first.SettlementTxID == "" {
		t.Fatal(
			"missing settlement transaction ID",
		)
	}

	if first.BountyReward != job.Reward {
		t.Fatalf(
			"unexpected bounty: got %d want %d",
			first.BountyReward,
			job.Reward,
		)
	}

	afterFirst, _, _, err :=
		api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	if len(afterFirst.Blocks) == 0 {
		t.Fatal(
			"chain empty after settlement",
		)
	}

	heightAfterFirst :=
		afterFirst.Blocks[len(afterFirst.Blocks)-1].Height

	supplyAfterFirst, err :=
		afterFirst.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}

	aliceAfterFirst, err :=
		afterFirst.BalanceOf(
			alice.Address,
		)
	if err != nil {
		t.Fatal(err)
	}

	bobAfterFirst, err :=
		afterFirst.BalanceOf(
			bob.Address,
		)
	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		settleComputeJob(
			api,
			job.ID,
			proof,
		)
	if err != nil {
		t.Fatal(err)
	}

	if !second.Recovered {
		t.Fatal(
			"retry must use settlement recovery",
		)
	}

	if second.SettlementTxID !=
		first.SettlementTxID {

		t.Fatalf(
			"settlement transaction changed: got %s want %s",
			second.SettlementTxID,
			first.SettlementTxID,
		)
	}

	if second.Block != first.Block {
		t.Fatalf(
			"settlement block changed: got %d want %d",
			second.Block,
			first.Block,
		)
	}

	afterRetry, _, _, err :=
		api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	heightAfterRetry :=
		afterRetry.Blocks[len(afterRetry.Blocks)-1].Height

	if heightAfterRetry != heightAfterFirst {
		t.Fatalf(
			"retry created another block: got height %d want %d",
			heightAfterRetry,
			heightAfterFirst,
		)
	}

	supplyAfterRetry, err :=
		afterRetry.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}

	if supplyAfterRetry != supplyAfterFirst {
		t.Fatalf(
			"retry changed total supply: got %d want %d",
			supplyAfterRetry,
			supplyAfterFirst,
		)
	}

	aliceAfterRetry, err :=
		afterRetry.BalanceOf(
			alice.Address,
		)
	if err != nil {
		t.Fatal(err)
	}

	if aliceAfterRetry != aliceAfterFirst {
		t.Fatalf(
			"retry changed Alice balance: got %d want %d",
			aliceAfterRetry,
			aliceAfterFirst,
		)
	}

	bobAfterRetry, err :=
		afterRetry.BalanceOf(
			bob.Address,
		)
	if err != nil {
		t.Fatal(err)
	}

	if bobAfterRetry != bobAfterFirst {
		t.Fatalf(
			"retry changed Bob balance: got %d want %d",
			bobAfterRetry,
			bobAfterFirst,
		)
	}
}
