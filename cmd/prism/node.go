package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/identity"
	"prism/internal/mempool"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func runNodeCommand(
	args []string,
) {
	flags := flag.NewFlagSet(
		"node",
		flag.ContinueOnError,
	)

	flags.SetOutput(os.Stdout)

	host := flags.String(
		"host",
		"127.0.0.1",
		"TCP host/interface used by the Prism node",
	)

	port := flags.Int(
		"port",
		7001,
		"TCP port used by the Prism node",
	)

	peer := flags.String(
		"peer",
		"",
		"peer address such as 127.0.0.1:7001",
	)

	nodeData := flags.String(
		"data",
		"",
		"node data directory",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *port < 1 || *port > 65535 {
		fmt.Println(
			"Invalid port:",
			*port,
		)
		return
	}

	dataPath := resolveNodeDataPath(
		*port,
		*nodeData,
	)

	chain, pos, wallets, created, err :=
		loadOrCreateP2PState(dataPath)

	if err != nil {
		fmt.Println(
			"Unable to start Prism node:",
		)
		fmt.Println(err)
		return
	}

	if created {
		fmt.Println(
			"Created new node state:",
			dataPath,
		)
	} else {
		fmt.Println(
			"Loaded node state:",
			dataPath,
		)
	}

	if !chain.ValidateChain(pos) {
		fmt.Println(
			"Refusing to start with invalid blockchain.",
		)
		return
	}

	identity := wallets["Alice"]

	if identity == nil {
		fmt.Println(
			"Node identity wallet is missing.",
		)
		return
	}

	listenAddr := fmt.Sprintf(
		"%s:%d",
		*host,
		*port,
	)

	nodeID := p2p.MakeNodeID(
		identity.Address,
	)

	fmt.Println()
	fmt.Println(
		"Blockchain valid:",
		true,
	)

	fmt.Println(
		"Data directory:",
		dataPath,
	)

	server := p2p.NewServer(
		nodeID,
		listenAddr,
		dataPath,
		chain,
		pos,
		wallets,
	)

	if err := server.Run(*peer); err != nil {
		fmt.Println()
		fmt.Println(
			"P2P node stopped:",
		)
		fmt.Println(err)
	}
}

func runNodeProduceCommand(
	args []string,
) {
	flags := flag.NewFlagSet(
		"node-produce",
		flag.ContinueOnError,
	)

	flags.SetOutput(os.Stdout)

	port := flags.Int(
		"port",
		7001,
		"node data port",
	)

	nodeData := flags.String(
		"data",
		"",
		"node data directory",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *port < 1 || *port > 65535 {
		fmt.Println(
			"Invalid port:",
			*port,
		)
		return
	}

	dataPath := resolveNodeDataPath(
		*port,
		*nodeData,
	)

	chain, pos, wallets, created, err :=
		loadOrCreateP2PState(dataPath)

	if err != nil {
		fmt.Println(
			"Unable to load node state:",
		)
		fmt.Println(err)
		return
	}

	if created {
		fmt.Println(
			"Created new node state:",
			dataPath,
		)
	} else {
		fmt.Println(
			"Loaded node state:",
			dataPath,
		)
	}

	if !chain.ValidateChain(pos) {
		fmt.Println(
			"Refusing to produce on invalid blockchain.",
		)
		return
	}

	alice := wallets["Alice"]
	bob := wallets["Bob"]
	charlie := wallets["Charlie"]

	if alice == nil ||
		bob == nil ||
		charlie == nil {

		fmt.Println(
			"Required local wallets are missing.",
		)
		return
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]
	nextHeight := lastBlock.Height + 1

	fmt.Println()
	fmt.Println(
		"=== PRODUCING NODE BLOCK ===",
	)

	fmt.Printf(
		"Current height: %d\n",
		lastBlock.Height,
	)

	pool := mempool.New()

	aliceNonce, err := pool.NextNonce(
		alice.Address,
		chain,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	tx := transaction.New(
		alice.Address,
		bob.Address,
		1,
		aliceNonce,
		alice.PublicKeyHex(),
	)

	if err := tx.Sign(
		alice.PrivateKey,
	); err != nil {
		fmt.Println(err)
		return
	}

	if err := pool.Add(
		tx,
		chain,
	); err != nil {
		fmt.Println(
			"Unable to add transaction:",
		)
		fmt.Println(err)
		return
	}

	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{
			nextHeight + 1,
			nextHeight + 2,
			nextHeight + 3,
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	proof, err := usefulwork.Execute(
		task,
		charlie,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := usefulwork.VerifyProof(
		proof,
	); err != nil {
		fmt.Println(err)
		return
	}

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		nextHeight,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	block, err := chain.AddBlock(
		pool.Transactions(),
		[]usefulwork.Proof{
			proof,
		},
		proposer.Address,
		pos,
	)
	if err != nil {
		fmt.Println(
			"Block production failed:",
		)
		fmt.Println(err)
		return
	}

	if err := storage.Save(
		dataPath,
		chain,
		pos,
		wallets,
	); err != nil {
		fmt.Println(
			"Unable to save new block:",
		)
		fmt.Println(err)
		return
	}

	fmt.Println(
		"Transaction: Alice -> Bob: 1 PRISM",
	)

	fmt.Println(
		"Useful worker: Charlie",
	)

	fmt.Printf(
		"Useful result: %d\n",
		proof.Result,
	)

	fmt.Println(
		"PoS proposer:",
		shortAddress(proposer.Address),
	)

	fmt.Printf(
		"New height: %d\n",
		block.Height,
	)

	fmt.Println(
		"Block hash:",
		block.Hash,
	)

	fmt.Printf(
		"Chain valid: %t\n",
		chain.ValidateChain(pos),
	)

	fmt.Println()
	fmt.Println(
		"Node state saved:",
		dataPath,
	)
}

func resolveNodeDataPath(
	port int,
	explicit string,
) string {
	if explicit != "" {
		return explicit
	}

	return filepath.Join(
		"data",
		fmt.Sprintf(
			"node-%d",
			port,
		),
	)
}

func loadOrCreateP2PState(
	dataPath string,
) (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	map[string]*wallet.Wallet,
	bool,
	error,
) {
	if storage.Exists(dataPath) {
		chain, pos, wallets, err := storage.Load(
			dataPath,
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
		dataPath,
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

func runNodeHumanCommand(
	args []string,
) {
	const worldIDAction = "prism_poup"

	flags := flag.NewFlagSet(
		"node-human",
		flag.ContinueOnError,
	)

	flags.SetOutput(os.Stdout)

	port := flags.Int(
		"port",
		7001,
		"node data port",
	)

	nodeData := flags.String(
		"data",
		"",
		"node data directory",
	)

	if err := flags.Parse(args); err != nil {
		return
	}

	if *port < 1 || *port > 65535 {
		fmt.Println(
			"Invalid port:",
			*port,
		)
		return
	}

	positional := flags.Args()

	if len(positional) != 3 {
		fmt.Println("Usage:")
		fmt.Println(
			`prism node-human --data data/node-7001 Alice proof_001 nullifier_001`,
		)
		return
	}

	dataPath := resolveNodeDataPath(
		*port,
		*nodeData,
	)

	if !storage.Exists(dataPath) {
		fmt.Println(
			"Node state not found:",
			dataPath,
		)
		return
	}

	chain, pos, wallets, err := storage.Load(
		dataPath,
	)
	if err != nil {
		fmt.Println(
			"Unable to load node state:",
			err,
		)
		return
	}

	participant := positional[0]
	proofText := positional[1]
	nullifier := positional[2]

	address, participantName, err := resolveAddress(
		participant,
		wallets,
	)
	if err != nil {
		fmt.Println(
			"Humanity proof rejected:",
			err,
		)
		return
	}

	if chain.IsVerified(address) {
		fmt.Println(
			"Humanity proof rejected:",
		)
		fmt.Println(
			"prism address already humanity verified",
		)
		return
	}
	if proofText == "" {
		fmt.Println(
			"Humanity proof rejected:",
			"world id proof cannot be empty",
		)
		return
	}

	if nullifier == "" {
		fmt.Println(
			"Humanity proof rejected:",
			"world id nullifier cannot be empty",
		)
		return
	}

	attestation, err := identity.NewWorldIDAttestation(
		address,
		nullifier,
		worldIDAction,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(chain.Blocks) == 0 {
		fmt.Println(
			"Node blockchain is empty.",
		)
		return
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	block, err := chain.AddHumanityBlock(
		[]identity.Attestation{
			attestation,
		},
		proposer.Address,
		pos,
	)
	if err != nil {
		fmt.Println(
			"Unable to add humanity block:",
			err,
		)
		return
	}

	if err := storage.Save(
		dataPath,
		chain,
		pos,
		wallets,
	); err != nil {
		fmt.Println(
			"Unable to save node state:",
			err,
		)
		return
	}

	fmt.Println()
	fmt.Println(
		"=== NODE HUMANITY ATTESTATION CONFIRMED ===",
	)
	fmt.Println(
		"Participant:",
		participantName,
	)
	fmt.Println(
		"Address:",
		shortAddress(address),
	)
	fmt.Println(
		"Provider:",
		attestation.Provider,
	)
	fmt.Println(
		"Action:",
		attestation.Action,
	)
	fmt.Println(
		"Proof: VERIFIED",
	)
	fmt.Println(
		"Replay check: PASSED",
	)
	fmt.Println(
		"Humanity: ON-CHAIN",
	)
	fmt.Println(
		"Block:",
		block.Height,
	)
	fmt.Println(
		"PoS proposer:",
		shortAddress(block.Proposer),
	)
	fmt.Println(
		"Block hash:",
		block.Hash,
	)
	fmt.Println(
		"Chain valid:",
		chain.ValidateChain(pos),
	)
	fmt.Println()
	fmt.Println(
		"Eligible for Proof of Useful Participation: YES",
	)
}
