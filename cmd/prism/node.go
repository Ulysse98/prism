package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func runNodeCommand(args []string) {
	flags := flag.NewFlagSet(
		"node",
		flag.ContinueOnError,
	)

	flags.SetOutput(os.Stdout)

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

	dataPath := *nodeData

	if dataPath == "" {
		dataPath = filepath.Join(
			"data",
			fmt.Sprintf(
				"node-%d",
				*port,
			),
		)
	}

	chain, pos, wallets, created, err :=
		loadOrCreateP2PState(dataPath)

	if err != nil {
		fmt.Println("Unable to start Prism node:")
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
		"127.0.0.1:%d",
		*port,
	)

	nodeID := p2p.MakeNodeID(
		identity.Address,
	)

	fmt.Println()
	fmt.Println("Blockchain valid:", true)
	fmt.Println("Data directory:", dataPath)

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
		fmt.Println("P2P node stopped:")
		fmt.Println(err)
	}
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
