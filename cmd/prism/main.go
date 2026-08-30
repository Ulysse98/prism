package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/mempool"
	"prism/internal/participation"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func main() {
	fmt.Println("====================================")
	fmt.Println("         PRISM NODE v0.11")
	fmt.Println("====================================")
	fmt.Println()

	// ============================================
	// WALLETS
	// ============================================

	alice, err := wallet.New()
	if err != nil {
		panic(err)
	}

	bob, err := wallet.New()
	if err != nil {
		panic(err)
	}

	charlie, err := wallet.New()
	if err != nil {
		panic(err)
	}

	fmt.Println("Participants:")
	fmt.Println("Alice:  ", shortAddress(alice.Address))
	fmt.Println("Bob:    ", shortAddress(bob.Address))
	fmt.Println("Charlie:", shortAddress(charlie.Address))

	// ============================================
	// GENESIS
	// ============================================

	initialBalances := map[string]uint64{
		alice.Address:   1200,
		bob.Address:     800,
		charlie.Address: 500,
	}

	chain, err := blockchain.NewBlockchain(
		initialBalances,
	)
	if err != nil {
		panic(err)
	}

	// ============================================
	// PROOF OF STAKE
	// ============================================

	pos := consensus.NewProofOfStake()

	registerValidator(
		chain,
		pos,
		alice.Address,
		500,
	)

	registerValidator(
		chain,
		pos,
		bob.Address,
		300,
	)

	registerValidator(
		chain,
		pos,
		charlie.Address,
		200,
	)

	fmt.Println()
	fmt.Println("=== PROOF OF STAKE ===")

	fmt.Println("Alice:   500 PRISM")
	fmt.Println("Bob:     300 PRISM")
	fmt.Println("Charlie: 200 PRISM")

	fmt.Printf(
		"Total stake: %d PRISM\n",
		pos.TotalStake(),
	)

	// ============================================
	// MEMPOOL
	// ============================================

	pool := mempool.New()

	aliceNonce, err := pool.NextNonce(
		alice.Address,
		chain,
	)
	if err != nil {
		panic(err)
	}

	tx1 := transaction.New(
		alice.Address,
		bob.Address,
		100,
		aliceNonce,
		alice.PublicKeyHex(),
	)

	if err := tx1.Sign(
		alice.PrivateKey,
	); err != nil {
		panic(err)
	}

	if err := pool.Add(
		tx1,
		chain,
	); err != nil {
		panic(err)
	}

	bobNonce, err := pool.NextNonce(
		bob.Address,
		chain,
	)
	if err != nil {
		panic(err)
	}

	tx2 := transaction.New(
		bob.Address,
		charlie.Address,
		50,
		bobNonce,
		bob.PublicKeyHex(),
	)

	if err := tx2.Sign(
		bob.PrivateKey,
	); err != nil {
		panic(err)
	}

	if err := pool.Add(
		tx2,
		chain,
	); err != nil {
		panic(err)
	}

	// ============================================
	// USEFUL WORK #1
	// ============================================

	fmt.Println()
	fmt.Println("=== USEFUL WORK #1 ===")

	task1, err := usefulwork.NewSumSquaresTask(
		[]uint64{
			3,
			4,
			5,
			12,
		},
	)
	if err != nil {
		panic(err)
	}

	proof1, err := usefulwork.Execute(
		task1,
		charlie,
	)
	if err != nil {
		panic(err)
	}

	if err := usefulwork.VerifyProof(
		proof1,
	); err != nil {
		panic(err)
	}

	fmt.Println(
		"Worker:",
		shortAddress(proof1.Worker),
	)

	fmt.Printf(
		"Result: %d\n",
		proof1.Result,
	)

	fmt.Printf(
		"Useful work score: %d\n",
		proof1.Score,
	)

	// ============================================
	// BLOCK #1
	// ============================================

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer1, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		panic(err)
	}

	block1, err := chain.AddBlock(
		pool.Transactions(),
		[]usefulwork.Proof{
			proof1,
		},
		proposer1.Address,
		pos,
	)
	if err != nil {
		panic(err)
	}

	pool.Clear()

	fmt.Println()
	fmt.Println("=== BLOCK #1 ===")

	fmt.Println(
		"Proposer:",
		shortAddress(block1.Proposer),
	)

	fmt.Println(
		"Useful worker:",
		shortAddress(proof1.Worker),
	)

	fmt.Printf(
		"Transactions: %d\n",
		len(block1.Transactions),
	)

	fmt.Println(
		"Hash:",
		block1.Hash,
	)

	// ============================================
	// USEFUL WORK #2
	// ============================================

	fmt.Println()
	fmt.Println("=== USEFUL WORK #2 ===")

	task2, err := usefulwork.NewSumSquaresTask(
		[]uint64{
			6,
			8,
			10,
		},
	)
	if err != nil {
		panic(err)
	}

	proof2, err := usefulwork.Execute(
		task2,
		bob,
	)
	if err != nil {
		panic(err)
	}

	if err := usefulwork.VerifyProof(
		proof2,
	); err != nil {
		panic(err)
	}

	fmt.Println(
		"Worker:",
		shortAddress(proof2.Worker),
	)

	fmt.Printf(
		"Result: %d\n",
		proof2.Result,
	)

	fmt.Printf(
		"Useful work score: %d\n",
		proof2.Score,
	)

	// ============================================
	// BLOCK #2
	// ============================================

	lastBlock = chain.Blocks[len(chain.Blocks)-1]

	proposer2, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		panic(err)
	}

	block2, err := chain.AddBlock(
		nil,
		[]usefulwork.Proof{
			proof2,
		},
		proposer2.Address,
		pos,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== BLOCK #2 ===")

	fmt.Println(
		"Proposer:",
		shortAddress(block2.Proposer),
	)

	fmt.Println(
		"Useful worker:",
		shortAddress(proof2.Worker),
	)

	fmt.Printf(
		"Transactions: %d\n",
		len(block2.Transactions),
	)

	fmt.Println(
		"Hash:",
		block2.Hash,
	)

	// ============================================
	// PROOF OF USEFUL PARTICIPATION
	// ============================================

	fmt.Println()
	fmt.Println("=== PROOF OF USEFUL PARTICIPATION ===")

	scores, err := participation.Calculate(
		chain,
		pos,
	)
	if err != nil {
		panic(err)
	}

	for index, score := range scores {
		fmt.Printf(
			"#%d %s\n",
			index+1,
			shortAddress(score.Address),
		)

		fmt.Printf(
			"   Blocks proposed: %d\n",
			score.BlocksProposed,
		)

		fmt.Printf(
			"   Useful work units: %d\n",
			score.UsefulWorkUnits,
		)

		fmt.Printf(
			"   Proposer score: %d\n",
			score.ProposerScore,
		)

		fmt.Printf(
			"   Useful work score: %d\n",
			score.UsefulWorkScore,
		)

		fmt.Printf(
			"   Participation score: %d\n",
			score.ParticipationScore,
		)
	}

	// ============================================
	// HUMAN-READABLE SCORES
	// ============================================

	fmt.Println()
	fmt.Println("=== PARTICIPATION BY ACCOUNT ===")

	fmt.Printf(
		"Alice: %d points\n",
		participation.ScoreOf(
			scores,
			alice.Address,
		),
	)

	fmt.Printf(
		"Bob: %d points\n",
		participation.ScoreOf(
			scores,
			bob.Address,
		),
	)

	fmt.Printf(
		"Charlie: %d points\n",
		participation.ScoreOf(
			scores,
			charlie.Address,
		),
	)

	// ============================================
	// SCORE FORGERY TEST
	// ============================================

	fmt.Println()
	fmt.Println("=== PARTICIPATION FORGERY TEST ===")

	originalScore := chain.Blocks[2].UsefulWork[0].Score

	chain.Blocks[2].UsefulWork[0].Score += 1000

	// L'attaquant recalcule même le hash du bloc.
	chain.Blocks[2].Hash = blockchain.CalculateHash(
		chain.Blocks[2],
	)

	_, err = participation.Calculate(
		chain,
		pos,
	)

	if err != nil {
		fmt.Println(
			"Forged participation rejected:",
		)
		fmt.Println(err)
	}

	// ============================================
	// RESTORE
	// ============================================

	chain.Blocks[2].UsefulWork[0].Score =
		originalScore

	chain.Blocks[2].Hash = blockchain.CalculateHash(
		chain.Blocks[2],
	)

	fmt.Printf(
		"Chain valid after restore: %t\n",
		chain.ValidateChain(pos),
	)

	restoredScores, err := participation.Calculate(
		chain,
		pos,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(
		"Participation state restored:",
		len(restoredScores) > 0,
	)

	// ============================================
	// FINAL STATE
	// ============================================

	fmt.Println()
	fmt.Println("=== FINAL ACCOUNT STATE ===")

	printAccount(
		"Alice",
		alice.Address,
		chain,
	)

	printAccount(
		"Bob",
		bob.Address,
		chain,
	)

	printAccount(
		"Charlie",
		charlie.Address,
		chain,
	)

	totalSupply, err := chain.TotalSupply()
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Printf(
		"Blocks: %d\n",
		len(chain.Blocks),
	)

	fmt.Printf(
		"Total supply: %d PRISM\n",
		totalSupply,
	)

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(pos),
	)

	fmt.Println()
	fmt.Println("Prism node is running.")
}

func registerValidator(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	address string,
	stake uint64,
) {

	if err := chain.LockStake(
		address,
		stake,
	); err != nil {
		panic(err)
	}

	if err := pos.Register(
		address,
		stake,
	); err != nil {

		if unlockErr := chain.UnlockStake(
			address,
			stake,
		); unlockErr != nil {
			panic(unlockErr)
		}

		panic(err)
	}
}

func printAccount(
	name string,
	address string,
	chain *blockchain.Blockchain,
) {

	balance, err := chain.BalanceOf(
		address,
	)
	if err != nil {
		panic(err)
	}

	available, err := chain.AvailableBalanceOf(
		address,
	)
	if err != nil {
		panic(err)
	}

	locked := chain.LockedStakeOf(
		address,
	)

	fmt.Printf(
		"%s: total=%d | locked=%d | available=%d PRISM\n",
		name,
		balance,
		locked,
		available,
	)
}

func shortAddress(
	address string,
) string {

	if len(address) <= 22 {
		return address
	}

	return address[:16] +
		"..." +
		address[len(address)-6:]
}
