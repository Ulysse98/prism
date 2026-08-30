package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/mempool"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func main() {
	fmt.Println("====================================")
	fmt.Println("         PRISM NODE v0.10")
	fmt.Println("====================================")
	fmt.Println()

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

	fmt.Println("=== PROOF OF STAKE ===")
	fmt.Println("Alice stake:   500 PRISM")
	fmt.Println("Bob stake:     300 PRISM")
	fmt.Println("Charlie stake: 200 PRISM")

	fmt.Printf(
		"Total stake: %d PRISM\n",
		pos.TotalStake(),
	)

	pool := mempool.New()

	// Alice -> Bob

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

	// Bob -> Charlie

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

	fmt.Println()
	fmt.Printf(
		"Mempool: %d transactions\n",
		pool.Count(),
	)

	// ============================================
	// PROOF OF USEFUL WORK
	// ============================================

	fmt.Println()
	fmt.Println("=== PROOF OF USEFUL WORK ===")

	task, err := usefulwork.NewSumSquaresTask(
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

	proof, err := usefulwork.Execute(
		task,
		charlie,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(
		"Task:",
		task.Type,
	)

	fmt.Println(
		"Worker:",
		shortAddress(proof.Worker),
	)

	fmt.Println(
		"Input: [3 4 5 12]",
	)

	fmt.Printf(
		"Result: %d\n",
		proof.Result,
	)

	fmt.Printf(
		"Work score: %d\n",
		proof.Score,
	)

	fmt.Printf(
		"Useful work reward: %d PRISM\n",
		blockchain.UsefulWorkReward,
	)

	if err := usefulwork.VerifyProof(
		proof,
	); err != nil {
		panic(err)
	}

	fmt.Println(
		"Proof verification: VALID",
	)

	// ============================================
	// SELECT PoS PROPOSER
	// ============================================

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== PoS PROPOSER ===")

	fmt.Println(
		"Selected proposer:",
		shortAddress(proposer.Address),
	)

	fmt.Printf(
		"Stake: %d PRISM\n",
		proposer.Stake,
	)

	// ============================================
	// FORGED USEFUL WORK TEST
	// ============================================

	fmt.Println()
	fmt.Println("=== FORGED USEFUL WORK TEST ===")

	tamperedProof := proof
	tamperedProof.Result++

	_, err = chain.AddBlock(
		pool.Transactions(),
		[]usefulwork.Proof{
			tamperedProof,
		},
		proposer.Address,
		pos,
	)

	if err != nil {
		fmt.Println(
			"Forged useful work rejected:",
		)
		fmt.Println(err)
	}

	// ============================================
	// VALID BLOCK
	// ============================================

	fmt.Println()
	fmt.Println("=== VALID HYBRID BLOCK ===")

	block, err := chain.AddBlock(
		pool.Transactions(),
		[]usefulwork.Proof{
			proof,
		},
		proposer.Address,
		pos,
	)
	if err != nil {
		panic(err)
	}

	pool.Clear()

	fmt.Printf(
		"Height: %d\n",
		block.Height,
	)

	fmt.Println(
		"PoS proposer:",
		shortAddress(block.Proposer),
	)

	fmt.Printf(
		"PoS reward: %d PRISM\n",
		block.Reward,
	)

	fmt.Printf(
		"Transactions: %d\n",
		len(block.Transactions),
	)

	fmt.Printf(
		"Useful work proofs: %d\n",
		len(block.UsefulWork),
	)

	fmt.Println(
		"Useful worker:",
		shortAddress(
			block.UsefulWork[0].Worker,
		),
	)

	fmt.Printf(
		"Useful result: %d\n",
		block.UsefulWork[0].Result,
	)

	fmt.Println(
		"Block hash:",
		block.Hash,
	)

	fmt.Println()
	fmt.Println("=== ACCOUNT STATE ===")

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

	fmt.Println()
	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(pos),
	)

	// ============================================
	// HISTORICAL PoUW TAMPER TEST
	// ============================================

	fmt.Println()
	fmt.Println("=== HISTORICAL PoUW TAMPER TEST ===")

	originalProof := chain.Blocks[1].UsefulWork[0]

	chain.Blocks[1].UsefulWork[0].Result++

	// Même en recalculant le hash du bloc,
	// le résultat de calcul reste faux.
	chain.Blocks[1].Hash = blockchain.CalculateHash(
		chain.Blocks[1],
	)

	fmt.Printf(
		"Chain valid after forged work: %t\n",
		chain.ValidateChain(pos),
	)

	chain.Blocks[1].UsefulWork[0] = originalProof

	chain.Blocks[1].Hash = blockchain.CalculateHash(
		chain.Blocks[1],
	)

	fmt.Printf(
		"Chain valid after restore: %t\n",
		chain.ValidateChain(pos),
	)

	totalSupply, err := chain.TotalSupply()
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Printf(
		"Total supply: %d PRISM\n",
		totalSupply,
	)

	fmt.Printf(
		"Mempool: %d transactions\n",
		pool.Count(),
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
