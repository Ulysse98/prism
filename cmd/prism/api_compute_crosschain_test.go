package main

import (
	"testing"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestSettleComputeJobBuildsCrossChainReceipt(
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

	if len(chain.Blocks) == 0 ||
		chain.Blocks[0].Hash == "" {

		t.Fatal("missing genesis block")
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
			[]uint64{13, 21, 34},
		)
	if err != nil {
		t.Fatal(err)
	}

	job, err := market.Create(
		task,
		"Alice",
		25,
		410001,
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

	chainID := p2p.MakeChainID(
		chain.Blocks[0].Hash,
	)

	proof, err :=
		usefulwork.ExecuteCompute(
			job.Task,
			job.ID,
			chainID,
			chain.Blocks[0].Hash,
			bob,
		)
	if err != nil {
		t.Fatal(err)
	}

	api := &apiServer{
		dataPath:      dataPath,
		computeMarket: market,
	}

	first, err := settleComputeJob(
		api,
		job.ID,
		proof,
	)
	if err != nil {
		t.Fatal(err)
	}

	if first.CrossChainReceipt == nil {
		t.Fatal("missing cross-chain receipt")
	}

	receipt := first.CrossChainReceipt

	if receipt.JobID != "0x"+job.ID {
		t.Fatalf(
			"unexpected receipt job ID: got %s want 0x%s",
			receipt.JobID,
			job.ID,
		)
	}

	if receipt.ProofID != "0x"+proof.ID {
		t.Fatalf(
			"unexpected receipt proof ID: got %s want 0x%s",
			receipt.ProofID,
			proof.ID,
		)
	}

	if receipt.WorkerIDHash == "" ||
		receipt.PrismChainIDHash == "" ||
		receipt.RegistryID == "" {

		t.Fatal(
			"cross-chain receipt contains empty hashes",
		)
	}

	second, err := settleComputeJob(
		api,
		job.ID,
		proof,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !second.Recovered {
		t.Fatal(
			"retry must recover existing settlement",
		)
	}

	if second.CrossChainReceipt == nil {
		t.Fatal(
			"recovered settlement lost cross-chain receipt",
		)
	}

	if second.CrossChainReceipt.RegistryID !=
		first.CrossChainReceipt.RegistryID {

		t.Fatalf(
			"registry ID changed on retry: got %s want %s",
			second.CrossChainReceipt.RegistryID,
			first.CrossChainReceipt.RegistryID,
		)
	}
}
