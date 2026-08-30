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
	fmt.Println("          PRISM NODE v0.9")
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

	fmt.Println("=== VALIDATOR SET ===")

	printValidator(
		"Alice",
		alice.Address,
		pos,
	)

	printValidator(
		"Bob",
		bob.Address,
		pos,
	)

	printValidator(
		"Charlie",
		charlie.Address,
		pos,
	)

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

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== CONSENSUS ===")

	fmt.Println(
		"Expected proposer:",
		shortAddress(proposer.Address),
	)

	fmt.Printf(
		"Stake: %d PRISM\n",
		proposer.Stake,
	)

	// ============================================
	// WRONG PROPOSER TEST
	// ============================================

	wrongProposer := chooseWrongProposer(
		proposer.Address,
		alice.Address,
		bob.Address,
		charlie.Address,
	)

	fmt.Println()
	fmt.Println("=== WRONG PROPOSER TEST ===")

	_, err = chain.AddBlock(
		pool.Transactions(),
		wrongProposer,
		pos,
	)

	if err != nil {
		fmt.Println("Forged proposer rejected:")
		fmt.Println(err)
	}

	// ============================================
	// CORRECT BLOCK
	// ============================================

	fmt.Println()
	fmt.Println("=== VALID BLOCK PRODUCTION ===")

	block, err := chain.AddBlock(
		pool.Transactions(),
		proposer.Address,
		pos,
	)
	if err != nil {
		panic(err)
	}

	pool.Clear()

	fmt.Printf(
		"Block height: %d\n",
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
	// HISTORICAL PROPOSER TAMPER TEST
	// ============================================

	fmt.Println()
	fmt.Println("=== HISTORICAL PROPOSER TAMPER TEST ===")

	originalProposer := chain.Blocks[1].Proposer

	chain.Blocks[1].Proposer = wrongProposer

	// Même si l'attaquant recalcule le hash,
	// le consensus PoS doit encore refuser le bloc.
	chain.Blocks[1].Hash = blockchain.CalculateHash(
		chain.Blocks[1],
	)

	fmt.Printf(
		"Chain valid after forged proposer: %t\n",
		chain.ValidateChain(pos),
	)

	// Restauration du bloc original.
	chain.Blocks[1].Proposer = originalProposer

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

func printValidator(
	name string,
	address string,
	pos *consensus.ProofOfStake,
) {

	fmt.Printf(
		"%s: %d PRISM staked\n",
		name,
		pos.StakeOf(address),
	)
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

func chooseWrongProposer(
	expected string,
	addresses ...string,
) string {

	for _, address := range addresses {
		if address != expected {
			return address
		}
	}

	panic(
		"unable to find alternative proposer",
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
