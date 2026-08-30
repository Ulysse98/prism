package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func main() {
	fmt.Println("====================================")
	fmt.Println("          PRISM NODE v0.5")
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

	hacker, err := wallet.New()
	if err != nil {
		panic(err)
	}

	fmt.Println("Wallets created:")
	fmt.Println("Alice:  ", alice.Address)
	fmt.Println("Bob:    ", bob.Address)
	fmt.Println("Charlie:", charlie.Address)

	initialBalances := map[string]uint64{
		alice.Address: 1000,
		bob.Address:   250,
	}

	chain, err := blockchain.NewBlockchain(initialBalances)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("Genesis balances:")

	printBalances(
		chain,
		alice.Address,
		bob.Address,
		charlie.Address,
	)

	// ============================================
	// SIGNED TRANSACTIONS
	// ============================================

	fmt.Println()
	fmt.Println("=== SIGNED BLOCK #1 ===")

	aliceNonce, err := chain.NonceOf(alice.Address)
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

	bobNonce, err := chain.NonceOf(bob.Address)
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

	_, err = chain.AddBlock(
		[]transaction.Transaction{
			tx1,
			tx2,
		},
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("Alice -> Bob: 125 PRISM [SIGNED]")
	fmt.Println("Bob -> Charlie: 50 PRISM [SIGNED]")

	fmt.Println()
	fmt.Println("Balances:")

	printBalances(
		chain,
		alice.Address,
		bob.Address,
		charlie.Address,
	)

	// ============================================
	// FORGED TRANSACTION TEST
	// ============================================

	fmt.Println()
	fmt.Println("=== FORGED TRANSACTION TEST ===")

	aliceNonce, err = chain.NonceOf(alice.Address)
	if err != nil {
		panic(err)
	}

	// Hacker prétend envoyer les PRISM d'Alice,
	// mais utilise sa propre clé.
	forgedTx := transaction.New(
		alice.Address,
		hacker.Address,
		100,
		aliceNonce,
		hacker.PublicKeyHex(),
	)

	if err := forgedTx.Sign(hacker.PrivateKey); err != nil {
		panic(err)
	}

	_, err = chain.AddBlock(
		[]transaction.Transaction{
			forgedTx,
		},
	)

	if err != nil {
		fmt.Println("Forged transaction rejected:")
		fmt.Println(err)
	}

	// ============================================
	// REPLAY ATTACK TEST
	// ============================================

	fmt.Println()
	fmt.Println("=== REPLAY ATTACK TEST ===")

	// On essaie de réutiliser l'ancienne transaction
	// d'Alice dont le nonce était 0.
	_, err = chain.AddBlock(
		[]transaction.Transaction{
			tx1,
		},
	)

	if err != nil {
		fmt.Println("Replay rejected:")
		fmt.Println(err)
	}

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

	fmt.Printf("Alice:   %d PRISM\n", aliceBalance)
	fmt.Printf("Bob:     %d PRISM\n", bobBalance)
	fmt.Printf("Charlie: %d PRISM\n", charlieBalance)
}
