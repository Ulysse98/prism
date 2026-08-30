package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/mempool"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func main() {
	fmt.Println("====================================")
	fmt.Println("          PRISM NODE v0.7")
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

	fmt.Println("Validators:")
	fmt.Println("Alice:  ", alice.Address)
	fmt.Println("Bob:    ", bob.Address)
	fmt.Println("Charlie:", charlie.Address)

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

	if err := pos.Register(
		alice.Address,
		500,
		chain,
	); err != nil {
		panic(err)
	}

	if err := pos.Register(
		bob.Address,
		300,
		chain,
	); err != nil {
		panic(err)
	}

	if err := pos.Register(
		charlie.Address,
		200,
		chain,
	); err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== PROOF OF STAKE ===")

	fmt.Println("Alice stake:   500 PRISM")
	fmt.Println("Bob stake:     300 PRISM")
	fmt.Println("Charlie stake: 200 PRISM")

	fmt.Printf(
		"Total stake:   %d PRISM\n",
		pos.TotalStake(),
	)

	// ============================================
	// MEMPOOL
	// ============================================

	pool := mempool.New()

	// Alice -> Bob : 100 PRISM

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

	if err := tx1.Sign(alice.PrivateKey); err != nil {
		panic(err)
	}

	if err := pool.Add(tx1, chain); err != nil {
		panic(err)
	}

	// Bob -> Charlie : 50 PRISM

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

	if err := tx2.Sign(bob.PrivateKey); err != nil {
		panic(err)
	}

	if err := pool.Add(tx2, chain); err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== MEMPOOL ===")

	fmt.Printf(
		"Transactions waiting: %d\n",
		pool.Count(),
	)

	fmt.Println("Alice -> Bob: 100 PRISM")
	fmt.Println("Bob -> Charlie: 50 PRISM")

	// ============================================
	// SELECT PROPOSER
	// ============================================

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	nextHeight := lastBlock.Height + 1

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		nextHeight,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== PROPOSER SELECTION ===")

	fmt.Println(
		"Selected validator:",
		shortAddress(proposer.Address),
	)

	fmt.Printf(
		"Validator stake: %d PRISM\n",
		proposer.Stake,
	)

	// ============================================
	// BLOCK PRODUCTION
	// ============================================

	block, err := chain.AddBlock(
		pool.Transactions(),
		proposer.Address,
	)
	if err != nil {
		panic(err)
	}

	pool.Clear()

	fmt.Println()
	fmt.Println("=== BLOCK PRODUCED ===")

	fmt.Printf(
		"Height: %d\n",
		block.Height,
	)

	fmt.Println(
		"Proposer:",
		shortAddress(block.Proposer),
	)

	fmt.Printf(
		"Transactions: %d\n",
		len(block.Transactions),
	)

	fmt.Println(
		"Hash:",
		block.Hash,
	)

	fmt.Println(
		"Previous hash:",
		block.PreviousHash,
	)

	// ============================================
	// FINAL BALANCES
	// ============================================

	fmt.Println()
	fmt.Println("=== FINAL BALANCES ===")

	printBalance(
		"Alice",
		alice.Address,
		chain,
	)

	printBalance(
		"Bob",
		bob.Address,
		chain,
	)

	printBalance(
		"Charlie",
		charlie.Address,
		chain,
	)

	// ============================================
	// FINAL VALIDATION
	// ============================================

	fmt.Println()

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(),
	)

	fmt.Printf(
		"Mempool transactions: %d\n",
		pool.Count(),
	)

	fmt.Println()
	fmt.Println("Prism node is running.")
}

func printBalance(
	name string,
	address string,
	chain *blockchain.Blockchain,
) {

	balance, err := chain.BalanceOf(address)
	if err != nil {
		panic(err)
	}

	fmt.Printf(
		"%s: %d PRISM\n",
		name,
		balance,
	)
}

func shortAddress(address string) string {
	if len(address) <= 22 {
		return address
	}

	return address[:16] +
		"..." +
		address[len(address)-6:]
}
