package main

import (
	"testing"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func TestQuantumMarketplaceEndToEndSettlement(
	t *testing.T,
) {
	// The unit/integration test must be deterministic and must not
	// require CUDA-Q, WSL or a GPU.
	t.Setenv(
		"PRISM_QUANTUM_BACKEND",
		"",
	)

	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}

	alice := wallets["Alice"]
	if alice == nil {
		t.Fatal(
			"Alice wallet missing",
		)
	}

	bob := wallets["Bob"]
	if bob == nil {
		t.Fatal(
			"Bob wallet missing",
		)
	}

	if len(chain.Blocks) == 0 ||
		chain.Blocks[0].Hash == "" {

		t.Fatal(
			"missing genesis block",
		)
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

	registerComputeMarketplaceCleanup(
		t,
		market,
	)

	task, err :=
		usefulwork.NewQuantumSimulationTask(
			2,
			4096,
		)
	if err != nil {
		t.Fatal(err)
	}

	if task.Type !=
		usefulwork.TaskTypeQuantumSimulation {

		t.Fatalf(
			"unexpected task type: %s",
			task.Type,
		)
	}

	job, err := market.Create(
		task,
		"Alice",
		31,
		540001,
	)
	if err != nil {
		t.Fatal(err)
	}

	if job.Status !=
		compute.JobStatusOpen {

		t.Fatalf(
			"expected OPEN quantum job, got %s",
			job.Status,
		)
	}

	job, err = market.Claim(
		job.ID,
		bob.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	if job.Status !=
		compute.JobStatusClaimed {

		t.Fatalf(
			"expected CLAIMED quantum job, got %s",
			job.Status,
		)
	}

	genesisHash :=
		chain.Blocks[0].Hash

	chainID :=
		p2p.MakeChainID(
			genesisHash,
		)

	proof, err :=
		usefulwork.ExecuteCompute(
			job.Task,
			job.ID,
			chainID,
			genesisHash,
			bob,
		)
	if err != nil {
		t.Fatal(err)
	}

	if proof.ProofVersion !=
		usefulwork.ComputeProofVersion {

		t.Fatalf(
			"expected compute proof version %d, got %d",
			usefulwork.ComputeProofVersion,
			proof.ProofVersion,
		)
	}

	if proof.JobID != job.ID {
		t.Fatalf(
			"proof job mismatch: got %s want %s",
			proof.JobID,
			job.ID,
		)
	}

	if len(proof.ResultValues) !=
		usefulwork.BellCountLength {

		t.Fatalf(
			"expected %d Bell counts, got %d",
			usefulwork.BellCountLength,
			len(proof.ResultValues),
		)
	}

	if err :=
		usefulwork.VerifyBellCounts(
			proof.ResultValues,
			4096,
		); err != nil {

		t.Fatalf(
			"invalid Bell counts: %v",
			err,
		)
	}

	if err :=
		usefulwork.VerifyComputeProofContext(
			proof,
			job.ID,
			chainID,
			genesisHash,
		); err != nil {

		t.Fatalf(
			"invalid quantum proof context: %v",
			err,
		)
	}

	if err :=
		usefulwork.VerifyProof(
			proof,
		); err != nil {

		t.Fatalf(
			"quantum proof rejected: %v",
			err,
		)
	}

	api := &apiServer{
		dataPath:      dataPath,
		computeMarket: market,
	}

	settlement, err :=
		settleComputeJob(
			api,
			job.ID,
			proof,
		)
	if err != nil {
		t.Fatal(err)
	}

	if settlement.Job.Status !=
		compute.JobStatusVerified {

		t.Fatalf(
			"expected VERIFIED quantum job, got %s",
			settlement.Job.Status,
		)
	}

	if settlement.Job.ProofID !=
		proof.ID {

		t.Fatalf(
			"proof ID mismatch: got %s want %s",
			settlement.Job.ProofID,
			proof.ID,
		)
	}

	if settlement.BountyReward !=
		job.Reward {

		t.Fatalf(
			"bounty mismatch: got %d want %d",
			settlement.BountyReward,
			job.Reward,
		)
	}

	if settlement.SettlementTxID == "" {
		t.Fatal(
			"missing quantum settlement transaction",
		)
	}

	if settlement.CrossChainReceipt == nil {
		t.Fatal(
			"missing quantum cross-chain receipt",
		)
	}

	if settlement.CrossChainReceipt.ProofID !=
		"0x"+proof.ID {

		t.Fatalf(
			"receipt proof mismatch: got %s want 0x%s",
			settlement.CrossChainReceipt.ProofID,
			proof.ID,
		)
	}

	if settlement.CrossChainReceipt.JobID !=
		"0x"+job.ID {

		t.Fatalf(
			"receipt job mismatch: got %s want 0x%s",
			settlement.CrossChainReceipt.JobID,
			job.ID,
		)
	}

	if settlement.CrossChainReceipt.RegistryID == "" {
		t.Fatal(
			"missing quantum receipt registry ID",
		)
	}

	storedJob, err :=
		market.Get(
			job.ID,
		)
	if err != nil {
		t.Fatal(err)
	}

	if storedJob.Status !=
		compute.JobStatusVerified {

		t.Fatalf(
			"persistent marketplace status mismatch: %s",
			storedJob.Status,
		)
	}

	if storedJob.ProofID !=
		proof.ID {

		t.Fatalf(
			"persistent marketplace proof mismatch: got %s want %s",
			storedJob.ProofID,
			proof.ID,
		)
	}

	finalChain, _, _, err :=
		api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	if !finalChain.HasRewardedUsefulWorkTask(
		task.ID,
	) {
		t.Fatal(
			"quantum PoUW task was not committed on-chain",
		)
	}

	heightBeforeRetry := finalChain.Blocks[len(finalChain.Blocks)-1].Height

	supplyBeforeRetry, err := finalChain.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}

	aliceBeforeRetry, err := finalChain.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}

	bobBeforeRetry, err := finalChain.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}

	retry, err := settleComputeJob(api, job.ID, proof)
	if err != nil {
		t.Fatalf("quantum settlement retry failed: %v", err)
	}

	if !retry.Recovered {
		t.Fatal("quantum retry must use recovery")
	}

	if retry.SettlementTxID != settlement.SettlementTxID {
		t.Fatal("quantum retry changed settlement transaction")
	}

	if retry.Block != settlement.Block {
		t.Fatal("quantum retry changed settlement block")
	}

	if retry.BountyReward != settlement.BountyReward {
		t.Fatal("quantum retry changed bounty reward")
	}

	afterRetry, _, _, err := api.loadState()
	if err != nil {
		t.Fatal(err)
	}

	heightAfterRetry := afterRetry.Blocks[len(afterRetry.Blocks)-1].Height
	if heightAfterRetry != heightBeforeRetry {
		t.Fatal("quantum retry created another block")
	}

	supplyAfterRetry, err := afterRetry.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	if supplyAfterRetry != supplyBeforeRetry {
		t.Fatal("quantum retry changed total supply")
	}

	aliceAfterRetry, err := afterRetry.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	if aliceAfterRetry != aliceBeforeRetry {
		t.Fatal("quantum retry changed requester balance")
	}

	bobAfterRetry, err := afterRetry.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}
	if bobAfterRetry != bobBeforeRetry {
		t.Fatal("quantum retry changed worker balance")
	}

}
