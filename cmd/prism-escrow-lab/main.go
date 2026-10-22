package main

import (
	"flag"
	"fmt"
	"os"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/escrow"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
)

func run() error {
	phase := flag.String("phase", "status", "fund, claim, release, refund, or status")
	data := flag.String("data", "", "existing test node data")
	peer := flag.String("peer", "", "receiver TCP address")
	listen := flag.String("listen", "127.0.0.1:17065", "sender advertised TCP address")
	expiryBlocks := flag.Uint64("expiry-blocks", 10, "blocks until FUND expires")
	flag.Parse()

	if *data == "" {
		return fmt.Errorf("-data is required")
	}
	if *phase == "fund" && (*expiryBlocks < 1 || *expiryBlocks > 100) {
		return fmt.Errorf("expiry-blocks must be between 1 and 100")
	}

	chain, pos, wallets, err := storage.Load(*data)
	if err != nil {
		return fmt.Errorf("load node: %w", err)
	}
	alice, bob := wallets["Alice"], wallets["Bob"]
	if alice == nil || bob == nil {
		return fmt.Errorf("Alice and Bob test wallets required")
	}
	if !chain.ValidateChain(pos) {
		return fmt.Errorf("invalid chain")
	}

	chainID, err := chain.ChainID()
	if err != nil {
		return err
	}
	task, err := usefulwork.NewSumSquaresTask([]uint64{3, 4, 5})
	if err != nil {
		return err
	}
	jobID := escrow.CalculateFundedJobID(
		task.ID, alice.Address, 10, 55,
	)

	if *phase == "status" {
		for _, block := range chain.Blocks {
			for _, auth := range block.EscrowFundings {
				jobID = auth.Funding.JobID
			}
		}
		record, err := chain.GetEscrowRecordV065(jobID)
		if err != nil {
			return err
		}
		spendable, err := chain.AvailableBalanceOf(
			record.Funding.Requester,
		)
		if err != nil {
			return err
		}
		workerBalance, err := chain.BalanceOf(record.Worker)
		if err != nil {
			return err
		}
		tip := chain.Blocks[len(chain.Blocks)-1]
		fmt.Printf(
			"height=%d hash=%s status=%s alice_spendable=%d worker_balance=%d job=%s\n",
			tip.Height, tip.Hash, record.Status,
			spendable, workerBalance, jobID,
		)
		return nil
	}

	if *peer == "" {
		return fmt.Errorf("-peer is required")
	}

	tip := chain.Blocks[len(chain.Blocks)-1]
	nextHeight := tip.Height + 1
	proposer, err := pos.SelectProposer(tip.Hash, nextHeight)
	if err != nil {
		return err
	}

	var block blockchain.Block

	switch *phase {
	case "fund":
		funding := escrow.Funding{
			ChainID:         chainID,
			GenesisHash:     chain.Blocks[0].Hash,
			JobID:           jobID,
			TaskID:          task.ID,
			JobNonce:        55,
			Requester:       alice.Address,
			Amount:          10,
			FundedAtHeight:  nextHeight,
			ExpiresAtHeight: nextHeight + *expiryBlocks,
		}
		auth, e := escrow.SignFunding(
			funding, 0, alice.PrivateKey,
		)
		if e != nil {
			return e
		}
		block, err = chain.AddEscrowFundingBlockV065(
			auth, proposer.Address, pos,
		)

	case "claim":
		claim, e := compute.SignClaimAuthorization(
			jobID, chainID, chain.Blocks[0].Hash, bob,
		)
		if e != nil {
			return e
		}
		block, err = chain.AddEscrowClaimBlockV065(
			claim, proposer.Address, pos,
		)

	case "release":
		proof, e := usefulwork.ExecuteCompute(
			task, jobID, chainID, chain.Blocks[0].Hash, bob,
		)
		if e != nil {
			return e
		}
		block, err = chain.AddEscrowReleaseBlockV065(
			proof, proposer.Address, pos,
		)

	case "refund":
		block, err = chain.AddEscrowRefundBlockV065(
			jobID, proposer.Address, pos,
		)

	default:
		return fmt.Errorf("unsupported phase: %s", *phase)
	}

	if err != nil {
		return fmt.Errorf("consensus rejected %s: %w", *phase, err)
	}
	if err := storage.Save(*data, chain, pos, wallets); err != nil {
		return fmt.Errorf("save source: %w", err)
	}

	server := p2p.NewServer(
		p2p.MakeNodeID(alice.Address),
		*listen,
		*data, chain, pos, wallets,
	)
	if err := server.SendBlock(*peer, block); err != nil {
		return fmt.Errorf(
			"saved locally but peer rejected block: %w", err,
		)
	}

	fmt.Printf(
		"ACK %s height=%d hash=%s job=%s\n",
		*phase, block.Height, block.Hash, jobID,
	)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
