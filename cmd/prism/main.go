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
	fmt.Println("          PRISM NODE v0.8")
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

	fmt.Println("=== LOCKED STAKING ===")

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

	fmt.Printf(
		"Total stake: %d PRISM\n",
		pos.TotalStake(),
	)

	pool := mempool.New()

	fmt.Println()
	fmt.Println("=== LOCKED STAKE SPEND TEST ===")

	aliceNonce, err := pool.NextNonce(
		alice.Address,
		chain,
	)
	if err != nil {
		panic(err)
	}

	overspend := transaction.New(
		alice.Address,
		bob.Address,
		800,
		aliceNonce,
		alice.PublicKeyHex(),
	)

	if err := overspend.Sign(
		alice.PrivateKey,
	); err != nil {
		panic(err)
	}

	err = pool.Add(
		overspend,
		chain,
	)

	if err != nil {
		fmt.Println("Transaction rejected:")
		fmt.Println(err)
	}

	fmt.Println()
	fmt.Println("=== VALID TRANSACTIONS ===")

	aliceNonce, err = pool.NextNonce(
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

	fmt.Printf(
		"Mempool: %d transactions\n",
		pool.Count(),
	)

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
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
		"Stake: %d PRISM\n",
		proposer.Stake,
	)

	fmt.Printf(
		"Block reward: %d PRISM\n",
		blockchain.BlockReward,
	)

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
		"Reward: %d PRISM\n",
		block.Reward,
	)

	fmt.Printf(
		"Transactions: %d\n",
		len(block.Transactions),
	)

	fmt.Println(
		"Hash:",
		block.Hash,
	)

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
		"Total supply: %d PRISM\n",
		totalSupply,
	)

	fmt.Printf(
		"Total locked stake: %d PRISM\n",
		pos.TotalStake(),
	)

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(),
	)

	fmt.Printf(
		"Mempool: %d transactions\n",
		pool.Count(),
	)

	fmt.Println()
	fmt.Println("Prism node is running.")
}

func printAccount(
	name string,
	address string,
	chain *blockchain.Blockchain,
) {

	balance, err := chain.BalanceOf(address)
	if err != nil {
		panic(err)
	}

	available, err := chain.AvailableBalanceOf(
		address,
	)
	if err != nil {
		panic(err)
	}

	locked := chain.LockedStakeOf(address)

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
