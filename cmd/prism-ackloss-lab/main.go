package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"prism/internal/blockchain"
	"prism/internal/p2p"
	"prism/internal/storage"
)

func run() error {
	data := flag.String("data", "", "source node data directory")
	peer := flag.String("peer", "", "destination TCP address")
	mode := flag.String("mode", "", "no-ack or replay")
	flag.Parse()

	if *data == "" || *peer == "" {
		return fmt.Errorf("-data and -peer are required")
	}
	if *mode != "no-ack" && *mode != "replay" {
		return fmt.Errorf("-mode must be no-ack or replay")
	}

	chain, pos, wallets, err := storage.Load(*data)
	if err != nil {
		return err
	}
	alice := wallets["Alice"]
	if alice == nil {
		return fmt.Errorf("missing node identity")
	}

	block := chain.Blocks[len(chain.Blocks)-1]
	if block.Height != 5 || len(block.EscrowReleases) != 1 {
		return fmt.Errorf("expected RELEASE block at height 5")
	}
	if blockchain.CalculateHash(block) != block.Hash {
		return fmt.Errorf("invalid source block hash")
	}

	server := p2p.NewServer(
		p2p.MakeNodeID(alice.Address),
		"127.0.0.1:17085",
		*data, chain, pos, wallets,
	)

	if *mode == "replay" {
		if err := server.SendBlock(*peer, block); err != nil {
			return err
		}
		fmt.Printf("REPLAY ACK ACCEPTED: %s\n", block.Hash)
		return nil
	}

	commitment := ""
	if !chain.Config.IsLegacy() {
		commitment, err = chain.Config.Commitment()
		if err != nil {
			return err
		}
	}
	chainID, err := blockchain.MakeChainIDFromConfigCommitment(
		chain.Blocks[0].Hash, commitment,
	)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", *peer, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	hello := p2p.HelloMessage{
		Type:             p2p.MessageHello,
		Version:          p2p.ProtocolVersion,
		NodeID:           p2p.MakeNodeID(alice.Address),
		ListenAddr:       "127.0.0.1:17085",
		ChainID:          chainID,
		ConfigCommitment: commitment,
		GenesisHash:      chain.Blocks[0].Hash,
		Height:           block.Height,
		LastHash:         block.Hash,
	}

	if err := encoder.Encode(hello); err != nil {
		return err
	}

	var remote p2p.HelloMessage
	if err := decoder.Decode(&remote); err != nil {
		return err
	}

	if remote.Version != p2p.ProtocolVersion ||
		remote.ChainID != chainID ||
		remote.NodeID == hello.NodeID ||
		remote.Height != 4 {
		return fmt.Errorf("unexpected receiver identity or height")
	}

	message := p2p.BlockMessage{
		Type:    p2p.MessageBlock,
		ChainID: chainID,
		Block:   block,
	}

	if err := encoder.Encode(message); err != nil {
		return err
	}

	// Deliberately close without decoding BlockAck.
	fmt.Printf(
		"RELEASE SENT WITHOUT READING ACK: %s\n",
		block.Hash,
	)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ACK-LOSS LAB ERROR:", err)
		os.Exit(1)
	}
}
