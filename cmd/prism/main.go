package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/mempool"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func main() {
	fmt.Println("====================================")
	fmt.Println("          PRISM NODE v0.6")
	fmt.Println("====================================")
	fmt.Println()

	// --------------------------------------------
	// WALLETS
	// --------------------------------------------

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

	fmt.Println("Wallets:")
	fmt.Println("Alice:  ", alice.Address)
	fmt.Println("Bob:    ", bob.Address)
	fmt.Println("Charlie:", charlie.Address)

	// --------------------------------------------
	// GENESIS
	// --------------------------------------------

	initialBalances := map[string]uint64{
		alice.Address: 1000,
		bob.Address:   250,
	}

	chain, err := blockchain.NewBlockchain(
		initialBalances,
	)
	if err != nil {
		panic(err)
	}

	pool := mempool.New()

	fmt.Println()
	fmt.Println("Genesis balances:")

	printBalances(
		chain,
		alice.Address,
		bob.Address,
		charlie.Address,
	)

	// --------------------------------------------
	// TRANSACTION #1
	// --------------------------------------------

	fmt.Println()
	fmt.Println("=== ADD TX #1 TO MEMPOOL ===")

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
		125,
		aliceNonce,
		alice.PublicKeyHex(),
	)

	if err := tx1.Sign(alice.PrivateKey); err != nil {
		panic(err)
	}

	if err := pool.Add(tx1, chain); err != nil {
		panic(err)
	}

	fmt.Println("Alice -> Bob: 125 PRISM")
	fmt.Printf(
		"Mempool transactions: %d\n",
		pool.Count(),
	)

	// --------------------------------------------
	// TRANSACTION #2
	// --------------------------------------------

	fmt.Println()
	fmt.Println("=== ADD TX #2 TO MEMPOOL ===")

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

	fmt.Println("Bob -> Charlie: 50 PRISM")

	fmt.Printf(
		"Mempool transactions: %d\n",
		pool.Count(),
	)

	// --------------------------------------------
	// DUPLICATE TEST
	// --------------------------------------------

	fmt.Println()
	fmt.Println("=== DUPLICATE TEST ===")

	err = pool.Add(tx1, chain)

	if err != nil {
		fmt.Println("Duplicate rejected:")
		fmt.Println(err)
	}

	// --------------------------------------------
	// PENDING OVERSPEND TEST
	// --------------------------------------------

	fmt.Println()
	fmt.Println("=== PENDING OVERSPEND TEST ===")

	aliceNonce, err = pool.NextNonce(
		alice.Address,
		chain,
	)
	if err != nil {
		panic(err)
	}

	overspend := transaction.New(
		alice.Address,
		charlie.Address,
		900,
		aliceNonce,
		alice.PublicKeyHex(),
	)

	if err := overspend.Sign(alice.PrivateKey); err != nil {
		panic(err)
	}

	err = pool.Add(
		overspend,
		chain,
	)

	if err != nil {
		fmt.Println("Overspend rejected:")
		fmt.Println(err)
	}

	// --------------------------------------------
	// BLOCK PRODUCTION
	// --------------------------------------------

	fmt.Println()
	fmt.Println("=== PRODUCE BLOCK ===")

	pendingTransactions := pool.Transactions()

	block, err := chain.AddBlock(
		pendingTransactions,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf(
		"Block #%d produced\n",
		block.Height,
	)

	fmt.Printf(
		"Transactions included: %d\n",
		len(block.Transactions),
	)

	// Les transactions sont maintenant confirmées.
	pool.Clear()

	fmt.Printf(
		"Mempool transactions after block: %d\n",
		pool.Count(),
	)

	// --------------------------------------------
	// FINAL STATE
	// --------------------------------------------

	fmt.Println()
	fmt.Println("Confirmed balances:")

	printBalances(
		chain,
		alice.Address,
		bob.Address,
		charlie.Address,
	)

	fmt.Println()
	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(),
	)

	fmt.Println()
	fmt.Println("Prism node is running.")
}

func printBalances(
	chain *blockchain.Blockchain,
	alice string,
	bob string,
	charlie string,
) {

	aliceBalance, err := chain.BalanceOf(alice)
	if err != nil {
		panic(err)
	}

	bobBalance, err := chain.BalanceOf(bob)
	if err != nil {
		panic(err)
	}

	charlieBalance, err := chain.BalanceOf(charlie)
	if err != nil {
		panic(err)
	}

	fmt.Printf(
		"Alice:   %d PRISM\n",
		aliceBalance,
	)

	fmt.Printf(
		"Bob:     %d PRISM\n",
		bobBalance,
	)

	fmt.Printf(
		"Charlie: %d PRISM\n",
		charlieBalance,
	)
}
