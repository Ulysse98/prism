package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

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
	fmt.Println("         PRISM NODE v0.14")
	fmt.Println("====================================")
	fmt.Println()

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := strings.ToLower(os.Args[1])

	// La commande node utilise son propre dossier de données
	// et est gérée dans cmd/prism/node.go.
	if command == "node" {
		runNodeCommand(os.Args[2:])
		return
	}

	chain, pos, wallets, created, err := loadOrCreateNode()
	if err != nil {
		panic(err)
	}

	if created {
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
	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	totalSupply, err := chain.TotalSupply()
	if err != nil {
		panic(err)
	}

	fmt.Println("=== PRISM STATUS ===")

	fmt.Printf(
		"Blocks:       %d\n",
		len(chain.Blocks),
	)

	fmt.Printf(
		"Height:       %d\n",
		lastBlock.Height,
	)

	fmt.Printf(
		"Validators:   %d\n",
		len(pos.Validators),
	)

	fmt.Printf(
		"Total stake:  %d PRISM\n",
		pos.TotalStake(),
	)

	fmt.Printf(
		"Total supply: %d PRISM\n",
		totalSupply,
	)

	fmt.Printf(
		"Chain valid:  %t\n",
		chain.ValidateChain(pos),
	)

	fmt.Println(
		"Last hash:",
		lastBlock.Hash,
	)
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

		fmt.Println(
			" ",
			currentWallet.Address,
		)
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

	fmt.Printf(
		"=== BALANCE: %s ===\n",
		label,
	)

	fmt.Println(
		"Address:",
		address,
	)

	fmt.Printf(
		"Total:     %d PRISM\n",
		total,
	)

	fmt.Printf(
		"Locked:    %d PRISM\n",
		locked,
	)

	fmt.Printf(
		"Available: %d PRISM\n",
		available,
	)

	fmt.Printf(
		"Nonce:     %d\n",
		nonce,
	)
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
	scores, err := participation.Calculate(
		chain,
		pos,
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

	if len(scores) == 0 {
		fmt.Println(
			"No participation recorded yet.",
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
			return name, currentWallet, nil
		}

		if currentWallet.Address == identifier {
			return name, currentWallet, nil
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
		`  .\prism.exe send Alice Bob 25`,
	)

	fmt.Println(
		`  .\prism.exe work Charlie 3 4 5`,
	)

	fmt.Println(
		`  .\prism.exe node --port 7001`,
	)

	fmt.Println(
		`  .\prism.exe node --port 7002 --peer 127.0.0.1:7001`,
	)

	fmt.Println(
		`  .\prism.exe help`,
	)
}
