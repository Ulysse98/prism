package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/identity"
	"prism/internal/mempool"
	"prism/internal/participation"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

const dataDir = "data"

func main() {
	jsonWorkLog := len(os.Args) == 3 &&
		strings.EqualFold(os.Args[1], "worklog") &&
		os.Args[2] == "--json"

	if !jsonWorkLog {
		fmt.Println("====================================")
		fmt.Println("         PRISM NODE v0.17")
		fmt.Println("====================================")
		fmt.Println()
	}

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := strings.ToLower(os.Args[1])

	// P2P node commands use their own data directories.
	if command == "node" {
		runNodeCommand(os.Args[2:])
		return
	}

	if command == "node-produce" {
		runNodeProduceCommand(os.Args[2:])
		return
	}

	chain, pos, wallets, created, err := loadOrCreateNode()
	if err != nil {
		panic(err)
	}

	if created && !jsonWorkLog {
		fmt.Println("New Prism state created.")
		fmt.Println()
	}

	switch command {
	case "status":
		runStatus(chain, pos)

	case "wallets":
		runWallets(wallets)

	case "balance":
		if len(os.Args) != 3 {
			fmt.Println("Usage:")
			fmt.Println(`.\prism.exe balance Alice`)
			return
		}

		runBalance(
			os.Args[2],
			chain,
			wallets,
		)

	case "validators":
		runValidators(
			chain,
			pos,
			wallets,
		)

	case "participation":
		runParticipation(
			chain,
			pos,
			wallets,
		)

	case "human":
		if len(os.Args) != 5 {
			fmt.Println("Usage:")
			fmt.Println(`.\prism.exe human Alice proof_001 nullifier_001`)
			return
		}

		if err := runHuman(
			os.Args[2],
			os.Args[3],
			os.Args[4],
			wallets,
		); err != nil {
			fmt.Println("Humanity proof rejected:")
			fmt.Println(err)
		}

	case "send":
		if len(os.Args) != 5 {
			fmt.Println("Usage:")
			fmt.Println(`.\prism.exe send Alice Bob 25`)
			return
		}

		if err := runSend(
			os.Args[2],
			os.Args[3],
			os.Args[4],
			chain,
			pos,
			wallets,
		); err != nil {
			fmt.Println("Transaction failed:")
			fmt.Println(err)
		}

	case "work":
		if len(os.Args) < 4 {
			fmt.Println("Usage:")
			fmt.Println(`.\prism.exe work Charlie 3 4 5`)
			return
		}

		if err := runUsefulWork(
			os.Args[2],
			os.Args[3:],
			chain,
			pos,
			wallets,
		); err != nil {
			fmt.Println("Useful Work failed:")
			fmt.Println(err)
		}

	case "worklog":
		if len(os.Args) == 2 {
			runWorkLog(
				chain,
				wallets,
			)
			return
		}

		if len(os.Args) != 3 || os.Args[2] != "--json" {
			fmt.Println("Usage:")
			fmt.Println(`.\prism.exe worklog`)
			fmt.Println(`.\prism.exe worklog --json`)
			return
		}

		if err := runWorkLogJSON(
			chain,
			wallets,
		); err != nil {
			fmt.Fprintln(
				os.Stderr,
				"Work log JSON failed:",
				err,
			)
		}

	case "help":
		printUsage()

	default:
		fmt.Printf(
			"Unknown command: %s\n\n",
			command,
		)

		printUsage()
	}
}

func runStatus(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) {
	if len(chain.Blocks) == 0 {
		fmt.Println("Blockchain is empty.")
		return
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	totalSupply, err := chain.TotalSupply()
	if err != nil {
		panic(err)
	}

	fmt.Println("=== PRISM STATUS ===")

	fmt.Printf("Blocks:       %d\n", len(chain.Blocks))
	fmt.Printf("Height:       %d\n", lastBlock.Height)
	fmt.Printf("Validators:   %d\n", len(pos.Validators))
	fmt.Printf("Total stake:  %d PRISM\n", pos.TotalStake())
	fmt.Printf("Total supply: %d PRISM\n", totalSupply)
	fmt.Printf("Chain valid:  %t\n", chain.ValidateChain(pos))
	fmt.Println("Last hash:", lastBlock.Hash)
}

func runWallets(
	wallets map[string]*wallet.Wallet,
) {
	fmt.Println("=== LOCAL WALLETS ===")

	names := []string{
		"Alice",
		"Bob",
		"Charlie",
	}

	for _, name := range names {
		currentWallet := wallets[name]

		if currentWallet == nil {
			continue
		}

		fmt.Println(name)
		fmt.Println(" ", currentWallet.Address)
	}
}

func runBalance(
	identifier string,
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) {
	address, label, err := resolveAddress(
		identifier,
		wallets,
	)
	if err != nil {
		fmt.Println("Balance lookup failed:")
		fmt.Println(err)
		return
	}

	total, err := chain.BalanceOf(address)
	if err != nil {
		panic(err)
	}

	available, err := chain.AvailableBalanceOf(address)
	if err != nil {
		panic(err)
	}

	locked := chain.LockedStakeOf(address)

	nonce, err := chain.NonceOf(address)
	if err != nil {
		panic(err)
	}

	fmt.Printf("=== BALANCE: %s ===\n", label)
	fmt.Println("Address:", address)
	fmt.Printf("Total:     %d PRISM\n", total)
	fmt.Printf("Locked:    %d PRISM\n", locked)
	fmt.Printf("Available: %d PRISM\n", available)
	fmt.Printf("Nonce:     %d\n", nonce)
}

func runValidators(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) {
	fmt.Println("=== VALIDATORS ===")

	for index, validator := range pos.Validators {
		name := walletNameForAddress(
			validator.Address,
			wallets,
		)

		total, err := chain.BalanceOf(
			validator.Address,
		)
		if err != nil {
			panic(err)
		}

		available, err := chain.AvailableBalanceOf(
			validator.Address,
		)
		if err != nil {
			panic(err)
		}

		fmt.Printf(
			"#%d %s\n",
			index+1,
			name,
		)

		fmt.Println(
			"   Address:",
			shortAddress(validator.Address),
		)

		fmt.Printf(
			"   Stake: %d PRISM\n",
			validator.Stake,
		)

		fmt.Printf(
			"   Total balance: %d PRISM\n",
			total,
		)

		fmt.Printf(
			"   Available: %d PRISM\n",
			available,
		)
	}

	fmt.Printf(
		"\nTotal stake: %d PRISM\n",
		pos.TotalStake(),
	)
}

func runParticipation(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) {
	humanRegistry, err := identity.NewHumanRegistry(
		filepath.Join(
			dataDir,
			"worldid-humans.json",
		),
	)
	if err != nil {
		fmt.Println(
			"Unable to load humanity registry:",
		)
		fmt.Println(err)
		return
	}

	scores, err := participation.Calculate(
		chain,
		pos,
		humanRegistry,
	)
	if err != nil {
		fmt.Println(
			"Unable to calculate participation:",
		)

		fmt.Println(err)
		return
	}

	fmt.Println(
		"=== PROOF OF USEFUL PARTICIPATION ===",
	)
	fmt.Println("Human verification: REQUIRED")
	fmt.Println()

	if len(scores) == 0 {
		fmt.Println(
			"No humanity-verified participation recorded yet.",
		)
		return
	}

	for index, score := range scores {
		name := walletNameForAddress(
			score.Address,
			wallets,
		)

		fmt.Printf(
			"#%d %s\n",
			index+1,
			name,
		)

		fmt.Println(
			"   Address:",
			shortAddress(score.Address),
		)

		fmt.Println(
			"   Humanity: VERIFIED",
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
}

func runHuman(
	participantIdentifier string,
	proofText string,
	nullifier string,
	wallets map[string]*wallet.Wallet,
) error {
	const worldIDAction = "prism_poup"

	address, participantName, err := resolveAddress(
		participantIdentifier,
		wallets,
	)
	if err != nil {
		return err
	}

	verifier, err := identity.NewWorldIDVerifier(
		filepath.Join(
			dataDir,
			"worldid-nullifiers.json",
		),
	)
	if err != nil {
		return err
	}

	proof := identity.WorldIDProof{
		Nullifier: nullifier,
		Proof:     proofText,
		Action:    worldIDAction,
	}

	if err := verifier.Verify(
		proof,
		worldIDAction,
	); err != nil {
		return err
	}

	humanRegistry, err := identity.NewHumanRegistry(
		filepath.Join(
			dataDir,
			"worldid-humans.json",
		),
	)
	if err != nil {
		return err
	}

	if err := humanRegistry.MarkVerified(address); err != nil {
		return err
	}

	fmt.Println("=== HUMANITY VERIFIED ===")
	fmt.Println()
	fmt.Println("Participant:", participantName)
	fmt.Println("Address:", shortAddress(address))
	fmt.Println("Provider: World ID")
	fmt.Println("Action:", worldIDAction)
	fmt.Println("Proof: VERIFIED")
	fmt.Println("Nullifier:", nullifier)
	fmt.Println("Replay check: PASSED")
	fmt.Println("Human registry: UPDATED")
	fmt.Println()
	fmt.Println("Eligible for Proof of Useful Participation: YES")

	return nil
}

func runSend(
	fromIdentifier string,
	toIdentifier string,
	amountText string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	fromName, sender, err := resolveLocalWallet(
		fromIdentifier,
		wallets,
	)
	if err != nil {
		return err
	}

	toAddress, toLabel, err := resolveAddress(
		toIdentifier,
		wallets,
	)
	if err != nil {
		return err
	}

	amount, err := strconv.ParseUint(
		amountText,
		10,
		64,
	)
	if err != nil {
		return fmt.Errorf(
			"invalid amount: %w",
			err,
		)
	}

	if amount == 0 {
		return fmt.Errorf(
			"amount must be greater than zero",
		)
	}

	pool := mempool.New()

	nonce, err := pool.NextNonce(
		sender.Address,
		chain,
	)
	if err != nil {
		return err
	}

	tx := transaction.New(
		sender.Address,
		toAddress,
		amount,
		nonce,
		sender.PublicKeyHex(),
	)

	if err := tx.Sign(
		sender.PrivateKey,
	); err != nil {
		return err
	}

	if err := pool.Add(
		tx,
		chain,
	); err != nil {
		return err
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		return err
	}

	block, err := chain.AddBlock(
		pool.Transactions(),
		nil,
		proposer.Address,
		pos,
	)
	if err != nil {
		return err
	}

	if err := storage.Save(
		dataDir,
		chain,
		pos,
		wallets,
	); err != nil {
		return err
	}

	fmt.Println(
		"=== TRANSACTION CONFIRMED ===",
	)

	fmt.Printf(
		"%s -> %s: %d PRISM\n",
		fromName,
		toLabel,
		amount,
	)

	fmt.Println(
		"Transaction ID:",
		tx.ID,
	)

	fmt.Printf(
		"Nonce: %d\n",
		tx.Nonce,
	)

	fmt.Printf(
		"Block: %d\n",
		block.Height,
	)

	fmt.Println(
		"Proposer:",
		walletNameForAddress(
			block.Proposer,
			wallets,
		),
	)

	fmt.Println(
		"Block hash:",
		block.Hash,
	)

	return nil
}

func runUsefulWork(
	workerIdentifier string,
	valueTexts []string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	workerName, workerWallet, err := resolveLocalWallet(
		workerIdentifier,
		wallets,
	)
	if err != nil {
		return err
	}

	values := make(
		[]uint64,
		0,
		len(valueTexts),
	)

	for _, valueText := range valueTexts {
		value, err := strconv.ParseUint(
			valueText,
			10,
			64,
		)
		if err != nil {
			return fmt.Errorf(
				"invalid work value %q: %w",
				valueText,
				err,
			)
		}

		values = append(
			values,
			value,
		)
	}

	task, err := usefulwork.NewSumSquaresTask(
		values,
	)
	if err != nil {
		return err
	}

	proof, err := usefulwork.Execute(
		task,
		workerWallet,
	)
	if err != nil {
		return err
	}

	if err := usefulwork.VerifyProof(
		proof,
	); err != nil {
		return err
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		return err
	}

	block, err := chain.AddBlock(
		nil,
		[]usefulwork.Proof{
			proof,
		},
		proposer.Address,
		pos,
	)
	if err != nil {
		return err
	}

	if err := storage.Save(
		dataDir,
		chain,
		pos,
		wallets,
	); err != nil {
		return err
	}

	fmt.Println(
		"=== USEFUL WORK CONFIRMED ===",
	)

	fmt.Println(
		"Worker:",
		workerName,
	)

	fmt.Println(
		"Task:",
		task.Type,
	)

	fmt.Println(
		"Task ID:",
		task.ID,
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
		"Worker reward: %d PRISM\n",
		blockchain.UsefulWorkReward,
	)

	fmt.Printf(
		"Block: %d\n",
		block.Height,
	)

	fmt.Println(
		"PoS proposer:",
		walletNameForAddress(
			block.Proposer,
			wallets,
		),
	)

	fmt.Printf(
		"PoS reward: %d PRISM\n",
		block.Reward,
	)

	fmt.Println(
		"Block hash:",
		block.Hash,
	)

	return nil
}

func runWorkLog(
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) {
	fmt.Println("=== USEFUL WORK HISTORY ===")
	fmt.Println()

	found := false

	for blockIndex := len(chain.Blocks) - 1; blockIndex >= 0; blockIndex-- {
		block := chain.Blocks[blockIndex]

		for proofIndex := len(block.UsefulWork) - 1; proofIndex >= 0; proofIndex-- {
			proof := block.UsefulWork[proofIndex]
			found = true

			fmt.Printf("Block:       %d\n", block.Height)

			fmt.Println(
				"Worker:",
				walletNameForAddress(
					proof.Worker,
					wallets,
				),
			)

			fmt.Println(
				"Address:",
				shortAddress(proof.Worker),
			)

			fmt.Println(
				"Task:",
				proof.Task.Type,
			)

			fmt.Println(
				"Task ID:",
				proof.Task.ID,
			)

			fmt.Printf(
				"Result:      %d\n",
				proof.Result,
			)

			fmt.Printf(
				"Score:       %d\n",
				proof.Score,
			)

			fmt.Printf(
				"Reward:      %d PRISM\n",
				blockchain.UsefulWorkReward,
			)

			if err := usefulwork.VerifyProof(proof); err != nil {
				fmt.Println("Proof:       INVALID")
				fmt.Println("Error:", err)
			} else {
				fmt.Println("Proof:       VERIFIED")
			}

			fmt.Println(
				"Output hash:",
				proof.OutputHash,
			)

			fmt.Println(
				"Proof ID:",
				proof.ID,
			)

			fmt.Println(
				"Block hash:",
				block.Hash,
			)

			fmt.Println()
		}
	}

	if !found {
		fmt.Println("No useful work recorded yet.")
	}
}

func runWorkLogJSON(
	chain *blockchain.Blockchain,
	wallets map[string]*wallet.Wallet,
) error {
	entries := make(
		[]map[string]any,
		0,
	)

	for blockIndex := len(chain.Blocks) - 1; blockIndex >= 0; blockIndex-- {
		block := chain.Blocks[blockIndex]

		for proofIndex := len(block.UsefulWork) - 1; proofIndex >= 0; proofIndex-- {
			proof := block.UsefulWork[proofIndex]

			verifyErr := usefulwork.VerifyProof(proof)

			entry := map[string]any{
				"block":          block.Height,
				"worker":         walletNameForAddress(proof.Worker, wallets),
				"worker_address": proof.Worker,
				"task":           proof.Task.Type,
				"task_id":        proof.Task.ID,
				"result":         proof.Result,
				"score":          proof.Score,
				"reward":         blockchain.UsefulWorkReward,
				"verified":       verifyErr == nil,
				"output_hash":    proof.OutputHash,
				"proof_id":       proof.ID,
				"block_hash":     block.Hash,
			}

			if verifyErr != nil {
				entry["verification_error"] = verifyErr.Error()
			}

			entries = append(
				entries,
				entry,
			)
		}
	}

	encoder := json.NewEncoder(os.Stdout)

	encoder.SetIndent(
		"",
		"  ",
	)

	return encoder.Encode(entries)
}

func loadOrCreateNode() (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	map[string]*wallet.Wallet,
	bool,
	error,
) {
	if storage.Exists(dataDir) {
		chain, pos, wallets, err := storage.Load(
			dataDir,
		)
		if err != nil {
			return nil, nil, nil, false, err
		}

		return chain,
			pos,
			wallets,
			false,
			nil
	}

	chain, pos, wallets, err := createNode()
	if err != nil {
		return nil, nil, nil, false, err
	}

	if err := storage.Save(
		dataDir,
		chain,
		pos,
		wallets,
	); err != nil {
		return nil, nil, nil, false, err
	}

	return chain,
		pos,
		wallets,
		true,
		nil
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

	return chain,
		pos,
		wallets,
		nil
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

func resolveLocalWallet(
	identifier string,
	wallets map[string]*wallet.Wallet,
) (
	string,
	*wallet.Wallet,
	error,
) {
	for name, currentWallet := range wallets {
		if currentWallet == nil {
			continue
		}

		if strings.EqualFold(
			name,
			identifier,
		) {
			return name,
				currentWallet,
				nil
		}

		if currentWallet.Address == identifier {
			return name,
				currentWallet,
				nil
		}
	}

	return "", nil, fmt.Errorf(
		"local wallet not found: %s",
		identifier,
	)
}

func resolveAddress(
	identifier string,
	wallets map[string]*wallet.Wallet,
) (
	string,
	string,
	error,
) {
	for name, currentWallet := range wallets {
		if currentWallet == nil {
			continue
		}

		if strings.EqualFold(
			name,
			identifier,
		) {
			return currentWallet.Address,
				name,
				nil
		}

		if currentWallet.Address == identifier {
			return currentWallet.Address,
				name,
				nil
		}
	}

	if isPrismAddress(identifier) {
		return identifier,
			shortAddress(identifier),
			nil
	}

	return "", "", fmt.Errorf(
		"unknown Prism wallet or address: %s",
		identifier,
	)
}

func isPrismAddress(
	address string,
) bool {
	const prefix = "prism_"

	if !strings.HasPrefix(
		address,
		prefix,
	) {
		return false
	}

	if len(address) != 46 {
		return false
	}

	encoded := address[len(prefix):]

	decoded, err := hex.DecodeString(
		encoded,
	)
	if err != nil {
		return false
	}

	return len(decoded) == 20
}

func walletNameForAddress(
	address string,
	wallets map[string]*wallet.Wallet,
) string {
	for name, currentWallet := range wallets {
		if currentWallet == nil {
			continue
		}

		if currentWallet.Address == address {
			return name
		}
	}

	return shortAddress(address)
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

func printUsage() {
	fmt.Println("Prism CLI")
	fmt.Println()
	fmt.Println("Commands:")

	fmt.Println(
		`  .\prism.exe status`,
	)

	fmt.Println(
		`  .\prism.exe wallets`,
	)

	fmt.Println(
		`  .\prism.exe balance Alice`,
	)

	fmt.Println(
		`  .\prism.exe validators`,
	)

	fmt.Println(
		`  .\prism.exe participation`,
	)

	fmt.Println(
		`  .\prism.exe human Alice proof_001 nullifier_001`,
	)

	fmt.Println(
		`  .\prism.exe send Alice Bob 25`,
	)

	fmt.Println(
		`  .\prism.exe work Charlie 3 4 5`,
	)

	fmt.Println(
		`  .\prism.exe worklog`,
	)

	fmt.Println(
		`  .\prism.exe worklog --json`,
	)

	fmt.Println(
		`  .\prism.exe node --port 7001`,
	)

	fmt.Println(
		`  .\prism.exe node --port 7002 --peer 127.0.0.1:7001`,
	)

	fmt.Println(
		`  .\prism.exe node-produce --port 7001`,
	)

	fmt.Println(
		`  .\prism.exe help`,
	)
}
