package blockchain

import (
	"testing"

	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/escrow"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func escrowLifecycleFixtureV065(t *testing.T, expiry uint64) (*Blockchain, *wallet.Wallet, *wallet.Wallet, usefulwork.Task) {
	t.Helper()
	bc, pos, alice, bob := escrowFundingFixtureV065(t)
	task, err := usefulwork.NewSumSquaresTask([]uint64{3, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	funding := escrow.Funding{
		ChainID: chainID, GenesisHash: bc.Blocks[0].Hash,
		TaskID: task.ID, JobNonce: 55, Requester: alice.Address,
		Amount: 10, FundedAtHeight: 1, ExpiresAtHeight: expiry,
	}
	funding.JobID = escrow.CalculateFundedJobID(funding.TaskID, funding.Requester, funding.Amount, funding.JobNonce)
	fundAuth, err := escrow.SignFunding(funding, 0, alice.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bc.AddEscrowFundingBlockV065(fundAuth, alice.Address, pos); err != nil {
		t.Fatal(err)
	}
	return bc, alice, bob, task
}

func TestEscrowV065ConsensusReleaseConservesFundsAndRejectsReplay(t *testing.T) {
	bc, alice, bob, task := escrowLifecycleFixtureV065(t, 10)
	pos := escrowTestPoSV065(t, alice)
	locked, err := bc.GetEscrowLockedBalancesV065()
	if err != nil || locked[alice.Address] != 10 {
		t.Fatalf("funding lock missing: %v %v", locked, err)
	}
	fundedBalance, _ := bc.BalanceOf(alice.Address)
	bobBalance, _ := bc.BalanceOf(bob.Address)
	beforeSupply, _ := bc.TotalSupply()
	chainID, _ := bc.ChainID()
	recordBefore, err := bc.GetEscrowRecordV065(escrow.CalculateFundedJobID(task.ID, alice.Address, 10, 55))
	if err != nil {
		t.Fatal(err)
	}
	claim, err := compute.SignClaimAuthorization(recordBefore.Funding.JobID, chainID, bc.Blocks[0].Hash, bob)
	if err != nil {
		t.Fatal(err)
	}
	claimBlock, err := bc.AddEscrowClaimBlockV065(claim, alice.Address, pos)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, recordBefore.Funding.JobID, chainID, bc.Blocks[0].Hash, bob)
	if err != nil {
		t.Fatal(err)
	}
	releaseBlock, err := bc.AddEscrowReleaseBlockV065(proof, alice.Address, pos)
	if err != nil {
		t.Fatal(err)
	}
	gotAlice, _ := bc.BalanceOf(alice.Address)
	gotBob, _ := bc.BalanceOf(bob.Address)
	if gotAlice != fundedBalance+claimBlock.Reward+releaseBlock.Reward-10 {
		t.Fatalf("requester accounting mismatch: %d", gotAlice)
	}
	if gotBob != bobBalance+10 {
		t.Fatalf("worker credit mismatch: %d", gotBob)
	}
	afterSupply, _ := bc.TotalSupply()
	if afterSupply != beforeSupply+claimBlock.Reward+releaseBlock.Reward {
		t.Fatalf("release changed total supply: before=%d after=%d", beforeSupply, afterSupply)
	}
	locked, err = bc.GetEscrowLockedBalancesV065()
	if err != nil || locked[alice.Address] != 0 {
		t.Fatalf("released job still locked: %v %v", locked, err)
	}
	record, err := bc.GetEscrowRecordV065(recordBefore.Funding.JobID)
	if err != nil || record.Status != escrow.Released || record.ProofID != proof.ID {
		t.Fatalf("wrong released record: %+v %v", record, err)
	}
	if _, err := bc.AddEscrowReleaseBlockV065(proof, alice.Address, pos); err == nil {
		t.Fatal("duplicate RELEASE accepted")
	}
	if _, err := bc.AddEscrowRefundBlockV065(recordBefore.Funding.JobID, alice.Address, pos); err == nil {
		t.Fatal("REFUND after RELEASE accepted")
	}
	if !bc.ValidateChain(pos) || len(bc.Blocks) != 4 {
		t.Fatal("rejected replay changed the chain")
	}
	clone := &Blockchain{
		Blocks:       append([]Block(nil), bc.Blocks...),
		LockedStakes: map[string]uint64{alice.Address: 10},
	}
	if !clone.ValidateChain(pos) {
		t.Fatal("restarted node rejected committed chain")
	}
	cloneLocked, err := clone.GetEscrowLockedBalancesV065()
	if err != nil || cloneLocked[alice.Address] != 0 {
		t.Fatal("restarted node did not reconstruct settlement")
	}
}

func escrowTestPoSV065(t *testing.T, alice *wallet.Wallet) *consensus.ProofOfStake {
	t.Helper()
	pos := consensus.NewProofOfStake()
	if err := pos.Register(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	return pos
}

func TestEscrowV065ConsensusRefundAndForgeries(t *testing.T) {
	bc, alice, bob, task := escrowLifecycleFixtureV065(t, 3)
	pos := escrowTestPoSV065(t, alice)
	chainID, _ := bc.ChainID()
	jobID := escrow.CalculateFundedJobID(task.ID, alice.Address, 10, 55)
	claim, err := compute.SignClaimAuthorization(jobID, chainID, bc.Blocks[0].Hash, bob)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bc.AddEscrowClaimBlockV065(claim, alice.Address, pos); err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, jobID, chainID, bc.Blocks[0].Hash, bob)
	if err != nil {
		t.Fatal(err)
	}
	// Expiry is exclusive for RELEASE. Invalid operations must be atomic.
	if _, err := bc.AddEscrowReleaseBlockV065(proof, alice.Address, pos); err == nil {
		t.Fatal("release at expiry accepted")
	}
	wrong := claim
	wrong.Signature = "invalid"
	if _, err := bc.AddEscrowClaimBlockV065(wrong, alice.Address, pos); err == nil {
		t.Fatal("invalid or duplicate claim accepted")
	}
	availableBefore, _ := bc.AvailableBalanceOf(alice.Address)
	grossBefore, _ := bc.BalanceOf(alice.Address)
	beforeSupply, _ := bc.TotalSupply()
	block, err := bc.AddEscrowRefundBlockV065(jobID, alice.Address, pos)
	if err != nil {
		t.Fatal(err)
	}
	availableAfter, _ := bc.AvailableBalanceOf(alice.Address)
	grossAfter, _ := bc.BalanceOf(alice.Address)
	if availableAfter != availableBefore+10+block.Reward || grossAfter != grossBefore+block.Reward {
		t.Fatalf("refund accounting inconsistent, spendable %d=>%d gross %d=>%d", availableBefore, availableAfter, grossBefore, grossAfter)
	}
	afterSupply, _ := bc.TotalSupply()
	if afterSupply != beforeSupply+block.Reward {
		t.Fatal("refund minted/burned PRISM")
	}
	if _, err := bc.AddEscrowRefundBlockV065(jobID, alice.Address, pos); err == nil {
		t.Fatal("double refund accepted")
	}
	if _, err := bc.AddEscrowReleaseBlockV065(proof, alice.Address, pos); err == nil {
		t.Fatal("release after refund accepted")
	}
	if !bc.ValidateChain(pos) {
		t.Fatal("refund chain invalid")
	}
}

func TestEscrowV065ConsensusRejectsForgedReleaseBeforeExpiry(t *testing.T) {
	bc, alice, bob, task := escrowLifecycleFixtureV065(t, 10)
	pos := escrowTestPoSV065(t, alice)
	chainID, _ := bc.ChainID()
	jobID := escrow.CalculateFundedJobID(task.ID, alice.Address, 10, 55)
	claim, err := compute.SignClaimAuthorization(jobID, chainID, bc.Blocks[0].Hash, bob)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bc.AddEscrowClaimBlockV065(claim, alice.Address, pos); err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, jobID, chainID, bc.Blocks[0].Hash, bob)
	if err != nil {
		t.Fatal(err)
	}
	forged := proof
	forged.Result++
	if _, err := bc.AddEscrowReleaseBlockV065(forged, alice.Address, pos); err == nil {
		t.Fatal("forged proof accepted")
	}
	if _, err := bc.AddEscrowRefundBlockV065(jobID, alice.Address, pos); err == nil {
		t.Fatal("early refund accepted")
	}
	if len(bc.Blocks) != 3 || !bc.ValidateChain(pos) {
		t.Fatal("invalid operation mutated the chain")
	}
}
