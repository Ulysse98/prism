package main

import (
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/mempool"
	"prism/internal/participation"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

const dataDir = "data"

func main() {
	fmt.Println("====================================")
	fmt.Println("         PRISM NODE v0.12")
	fmt.Println("====================================")
	fmt.Println()

	var chain *blockchain.Blockchain
	var pos *consensus.ProofOfStake
	var wallets map[string]*wallet.Wallet
	var err error

	if storage.Exists(dataDir) {
		fmt.Println("Existing Prism state detected.")
		fmt.Println("Loading blockchain from disk...")

		chain, pos, wallets, err = storage.Load(
			dataDir,
		)
		if err != nil {
			panic(err)
		}

		fmt.Println("Blockchain loaded successfully.")
	} else {
		fmt.Println("No existing Prism state.")
		fmt.Println("Creating new blockchain...")

		chain, pos, wallets, err = createNode()
		if err != nil {
			panic(err)
		}

		if err := storage.Save(
			dataDir,
			chain,
			pos,
			wallets,
		); err != nil {
			panic(err)
		}

		fmt.Println("Genesis state saved to disk.")
	}

	alice := wallets["Alice"]
	bob := wallets["Bob"]
	charlie := wallets["Charlie"]

	if alice == nil ||
		bob == nil ||
		charlie == nil {

		panic(
			"required wallets are missing",
		)
	}

	fmt.Println()
	fmt.Println("=== LOADED STATE ===")

	fmt.Printf(
		"Blocks currently stored: %d\n",
		len(chain.Blocks),
	)

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	fmt.Printf(
		"Current height: %d\n",
		lastBlock.Height,
	)

	fmt.Println(
		"Current hash:",
		lastBlock.Hash,
	)

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(pos),
	)

	fmt.Println()
	fmt.Println("Wallets:")

	fmt.Println(
		"Alice:  ",
		shortAddress(alice.Address),
	)

	fmt.Println(
		"Bob:    ",
		shortAddress(bob.Address),
	)

	fmt.Println(
		"Charlie:",
		shortAddress(charlie.Address),
	)

	// ============================================
	// CREATE ONE NEW TRANSACTION
	// ============================================

	pool := mempool.New()

	fmt.Println()
	fmt.Println("=== NEW TRANSACTION ===")

	aliceNonce, err := pool.NextNonce(
		alice.Address,
		chain,
	)
	if err != nil {
		panic(err)
	}

	tx := transaction.New(
		alice.Address,
		bob.Address,
		10,
		aliceNonce,
		alice.PublicKeyHex(),
	)

	if err := tx.Sign(
		alice.PrivateKey,
	); err != nil {
		panic(err)
	}

	if err := pool.Add(
		tx,
		chain,
	); err != nil {
		panic(err)
	}

	fmt.Println(
		"Alice -> Bob: 10 PRISM",
	)

	fmt.Printf(
		"Nonce: %d\n",
		tx.Nonce,
	)

	// ============================================
	// CREATE UNIQUE USEFUL WORK
	// ============================================

	fmt.Println()
	fmt.Println("=== NEW USEFUL WORK ===")

	nextHeight := lastBlock.Height + 1

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{
			nextHeight,
			nextHeight + 1,
			nextHeight + 2,
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

	if err := usefulwork.VerifyProof(
		proof,
	); err != nil {
		panic(err)
	}

	fmt.Println(
		"Worker:",
		shortAddress(proof.Worker),
	)

	fmt.Println(
		"Task:",
		task.Type,
	)

	fmt.Printf(
		"Result: %d\n",
		proof.Result,
	)

	fmt.Printf(
		"Work score: %d\n",
		proof.Score,
	)

	// ============================================
	// PoS PROPOSER
	// ============================================

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		nextHeight,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== PROPOSER ===")

	fmt.Println(
		"Selected:",
		shortAddress(proposer.Address),
	)

	fmt.Printf(
		"Stake: %d PRISM\n",
		proposer.Stake,
	)

	// ============================================
	// PRODUCE NEW BLOCK
	// ============================================

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

	fmt.Printf(
		"Useful work proofs: %d\n",
		len(block.UsefulWork),
	)

	fmt.Println(
		"Hash:",
		block.Hash,
	)

	// ============================================
	// SAVE EVERYTHING
	// ============================================

	if err := storage.Save(
		dataDir,
		chain,
		pos,
		wallets,
	); err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("State saved to disk.")

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

	scores, err := participation.Calculate(
		chain,
		pos,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=== PARTICIPATION ===")

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
		"Height: %d\n",
		block.Height,
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
	fmt.Println(
		"Run Prism again to prove persistence.",
	)

	fmt.Println()
	fmt.Println("Prism node is running.")
}

func createNode() (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	map[string]*wallet.Wallet,
	error,
) {

	alice, err := wallet.New()
	if err != nil {
		return nil, nil, nil, err
	}

	bob, err := wallet.New()
	if err != nil {
		return nil, nil, nil, err
	}

	charlie, err := wallet.New()
	if err != nil {
		return nil, nil, nil, err
	}

	wallets := map[string]*wallet.Wallet{
		"Alice":   alice,
		"Bob":     bob,
		"Charlie": charlie,
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
		return nil, nil, nil, err
	}

	pos := consensus.NewProofOfStake()

	if err := registerValidator(
		chain,
		pos,
		alice.Address,
		500,
	); err != nil {
		return nil, nil, nil, err
	}

	if err := registerValidator(
		chain,
		pos,
		bob.Address,
		300,
	); err != nil {
		return nil, nil, nil, err
	}

	if err := registerValidator(
		chain,
		pos,
		charlie.Address,
		200,
	); err != nil {
		return nil, nil, nil, err
	}

	return chain, pos, wallets, nil
}

func registerValidator(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	address string,
	stake uint64,
) error {

	if err := chain.LockStake(
		address,
		stake,
	); err != nil {
		return err
	}

	if err := pos.Register(
		address,
		stake,
	); err != nil {

		if unlockErr := chain.UnlockStake(
			address,
			stake,
		); unlockErr != nil {
			return unlockErr
		}

		return err
	}

	return nil
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
