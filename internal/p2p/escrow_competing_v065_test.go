package p2p

import (
	"sync"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/escrow"
	"prism/internal/storage"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func escrowCompetingFixtureV065(t *testing.T, expiry uint64) (
	*blockchain.Blockchain, *consensus.ProofOfStake,
	*wallet.Wallet, *wallet.Wallet, usefulwork.Proof, string,
) {
	t.Helper()

	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := blockchain.NewBlockchain(map[string]uint64{
		alice.Address: 100,
		bob.Address:   5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.LockStake(alice.Address, 10); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()
	if err := pos.Register(alice.Address, 10); err != nil {
		t.Fatal(err)
	}

	task, err := usefulwork.NewSumSquaresTask([]uint64{3, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	chainID, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	funding := escrow.Funding{
		ChainID:         chainID,
		GenesisHash:     chain.Blocks[0].Hash,
		TaskID:          task.ID,
		JobNonce:        55,
		Requester:       alice.Address,
		Amount:          10,
		FundedAtHeight:  1,
		ExpiresAtHeight: expiry,
	}
	funding.JobID = escrow.CalculateFundedJobID(
		task.ID, alice.Address, funding.Amount, funding.JobNonce,
	)

	auth, err := escrow.SignFunding(funding, 0, alice.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chain.AddEscrowFundingBlockV065(
		auth, alice.Address, pos,
	); err != nil {
		t.Fatal(err)
	}

	claim, err := compute.SignClaimAuthorization(
		funding.JobID, chainID, chain.Blocks[0].Hash, bob,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chain.AddEscrowClaimBlockV065(
		claim, alice.Address, pos,
	); err != nil {
		t.Fatal(err)
	}

	proof, err := usefulwork.ExecuteCompute(
		task, funding.JobID, chainID, chain.Blocks[0].Hash, bob,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !chain.ValidateChain(pos) {
		t.Fatal("invalid CLAIMED fixture")
	}

	return chain, pos, alice, bob, proof, funding.JobID
}

func cloneEscrowTipV065(source *blockchain.Blockchain) *blockchain.Blockchain {
	locks := make(map[string]uint64, len(source.LockedStakes))
	for address, value := range source.LockedStakes {
		locks[address] = value
	}
	return &blockchain.Blockchain{
		Blocks:       append([]blockchain.Block(nil), source.Blocks...),
		LockedStakes: locks,
		Config:       source.Config,
	}
}

func TestEscrowV065P2PConcurrentDistinctReleaseCandidates(t *testing.T) {
	base, pos, alice, bob, proof, jobID :=
		escrowCompetingFixtureV065(t, 10)

	left := cloneEscrowTipV065(base)
	right := cloneEscrowTipV065(base)

	first, err := left.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Millisecond)

	second, err := right.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	if first.Height != second.Height ||
		first.Height != 3 || first.Hash == second.Hash {
		t.Fatal("expected two distinct valid RELEASE blocks")
	}
	if !left.ValidateChain(pos) || !right.ValidateChain(pos) {
		t.Fatal("candidate chain invalid")
	}

	dir := t.TempDir()

	receiver := NewServer(
		"escrow-race-receiver",
		"127.0.0.1:0",
		dir,
		cloneEscrowTipV065(base),
		pos,
		map[string]*wallet.Wallet{
			"Alice": alice,
			"Bob":   bob,
		},
	)

	const attempts = 16

	type result struct {
		blockHash string
		appended  bool
		err       error
	}
	results := make([]result, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < attempts; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			block := first
			if i%2 == 1 {
				block = second
			}

			<-start
			appended, err := receiver.acceptBlock(block)
			results[i] = result{block.Hash, appended, err}
		}(i)
	}

	close(start)
	wg.Wait()

	if len(receiver.Chain.Blocks) != len(base.Blocks)+1 {
		t.Fatal("concurrent submissions changed block count")
	}

	winner := receiver.Chain.Blocks[len(receiver.Chain.Blocks)-1]
	if winner.Hash != first.Hash && winner.Hash != second.Hash {
		t.Fatal("unexpected winning block")
	}

	accepted, known, rejected := 0, 0, 0

	for _, outcome := range results {
		switch {
		case outcome.blockHash == winner.Hash &&
			outcome.err == nil && outcome.appended:
			accepted++
		case outcome.blockHash == winner.Hash &&
			outcome.err == nil && !outcome.appended:
			known++
		case outcome.blockHash != winner.Hash &&
			outcome.err != nil && !outcome.appended:
			rejected++
		default:
			t.Fatalf("unexpected result: %+v", outcome)
		}
	}

	if accepted != 1 || known != 7 || rejected != 8 {
		t.Fatalf(
			"accepted=%d duplicates=%d rejected_forks=%d",
			accepted, known, rejected,
		)
	}

	if !receiver.Chain.ValidateChain(pos) {
		t.Fatal("invalid winner chain")
	}

	record, err := receiver.Chain.GetEscrowRecordV065(jobID)
	if err != nil || record.Status != escrow.Released {
		t.Fatalf("incorrect escrow state: %+v %v", record, err)
	}

	beforeBob, err := base.BalanceOf(bob.Address)
	if err != nil {
		t.Fatal(err)
	}
	afterBob, err := receiver.Chain.BalanceOf(bob.Address)
	if err != nil || afterBob != beforeBob+10 {
		t.Fatalf("incorrect worker payment: %d -> %d: %v",
			beforeBob, afterBob, err)
	}

	beforeSupply, err := base.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	afterSupply, err := receiver.Chain.TotalSupply()
	if err != nil || afterSupply != beforeSupply+winner.Reward {
		t.Fatalf("unexpected token supply: %d -> %d: %v",
			beforeSupply, afterSupply, err)
	}

	restored, restoredPoS, _, err := storage.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.ValidateChain(restoredPoS) {
		t.Fatal("persisted winner cannot be validated")
	}

	if len(restored.Blocks) != len(receiver.Chain.Blocks) {
		t.Fatal("persisted block count differs")
	}
	for i := range restored.Blocks {
		if restored.Blocks[i].Hash != receiver.Chain.Blocks[i].Hash {
			t.Fatalf("persisted hash differs at height %d", i)
		}
	}

	restoredBob, err := restored.BalanceOf(bob.Address)
	if err != nil || restoredBob != afterBob {
		t.Fatal("worker balance changed after reload")
	}

	t.Logf(
		"1 accepted, 7 duplicates, 8 conflicting blocks rejected; hash=%s",
		winner.Hash,
	)
}

func TestEscrowV065P2PExpiryBoundaryIsExclusive(t *testing.T) {
	chain, pos, alice, _, proof, jobID :=
		escrowCompetingFixtureV065(t, 3)

	initial := len(chain.Blocks)

	if _, err := chain.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	); err == nil {
		t.Fatal("RELEASE at expiry height must be rejected")
	}
	if len(chain.Blocks) != initial {
		t.Fatal("failed RELEASE mutated chain")
	}

	if _, err := chain.AddEscrowRefundBlockV065(
		jobID, alice.Address, pos,
	); err != nil {
		t.Fatalf("REFUND at expiry must succeed: %v", err)
	}

	record, err := chain.GetEscrowRecordV065(jobID)
	if err != nil || record.Status != escrow.Refunded {
		t.Fatalf("incorrect expiry transition: %+v %v", record, err)
	}
}
