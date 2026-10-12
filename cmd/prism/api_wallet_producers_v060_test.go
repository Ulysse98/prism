package main

import (
	"net/http"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
)

// A new testnet faucet payout MUST use the configured chain identity even on
// networks where it differs from the legacy PoUW proof chain ID.
func TestFaucetEmitsNetworkBoundV2(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "legacy_network"
		if configured {
			name = "configured_network"
		}
		t.Run(name, func(t *testing.T) {
			chain, pos, wallets, err := createNode()
			if err != nil {
				t.Fatal(err)
			}
			if configured {
				chain.Config = blockchain.ChainConfig{
					ReservedAuthorities: reserved.AuthorityPolicy{
						Treasury: []string{wallets["Alice"].Address},
					},
				}
			}
			expectedID, err := chain.ChainID()
			if err != nil {
				t.Fatal(err)
			}
			genesisHash := chain.Blocks[0].Hash
			dir := t.TempDir()
			if err := storage.Save(dir, chain, pos, wallets); err != nil {
				t.Fatal(err)
			}
			api := &apiServer{
				dataPath: dir, faucetEnabled: true,
				faucetWallet: "Alice", faucetAmount: 100,
				computeMarket: compute.NewMarketplace(),
			}
			receipt, status, err := api.dispenseFaucet(wallets["Bob"].Address)
			if err != nil || status != http.StatusCreated {
				t.Fatalf("faucet: HTTP %d, %v", status, err)
			}
			updated, updatedPoS, _, err := storage.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !updated.ValidateChain(updatedPoS) {
				t.Fatal("chain invalid after v2 faucet payout")
			}
			found := false
			for _, block := range updated.Blocks {
				for _, tx := range block.Transactions {
					if tx.ID != receipt.TxID {
						continue
					}
					found = true
					if tx.Version != transaction.VersionV2 ||
						tx.ChainID != expectedID || tx.GenesisHash != genesisHash {
						t.Fatalf("faucet emitted wrong signing context: %+v", tx)
					}
					if err := updated.ValidateTransactionContext(tx); err != nil {
						t.Fatalf("faucet tx context rejected: %v", err)
					}
				}
			}
			if !found {
				t.Fatal("faucet payout not found on chain")
			}
		})
	}
}

// Old blocks are still valid and claims made through v1 must not be reissued
// just because new faucet payouts are now v2.
func TestFaucetStillRecognizesLegacyV1Claim(t *testing.T) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := wallets["Alice"], wallets["Bob"]
	tx := transaction.New(alice.Address, bob.Address, 100, 0, alice.PublicKeyHex())
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	tip := chain.Blocks[len(chain.Blocks)-1]
	proposer, err := pos.SelectProposer(tip.Hash, tip.Height+1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chain.AddBlock([]transaction.Transaction{tx}, nil, proposer.Address, pos); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := storage.Save(dir, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}
	api := &apiServer{
		dataPath: dir, faucetEnabled: true,
		faucetWallet: "Alice", faucetAmount: 100,
		computeMarket: compute.NewMarketplace(),
	}
	_, status, err := api.dispenseFaucet(bob.Address)
	if status != http.StatusConflict || err == nil {
		t.Fatalf("v1 claim was not recognized: HTTP %d, %v", status, err)
	}
	saved, savedPoS, _, err := storage.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.ValidateChain(savedPoS) || saved.Blocks[1].Transactions[0].Version != 0 {
		t.Fatal("legacy history changed or became invalid")
	}
}

func TestFaucetPreservesFundedComputeBounties(t *testing.T) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	alice := wallets["Alice"]
	available, err := chain.AvailableBalanceOf(alice.Address)
	if err != nil || available < 150 {
		t.Fatalf("invalid test funds: %d %v", available, err)
	}
	dir := t.TempDir()
	if err := storage.Save(dir, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}
	market := compute.NewMarketplace()
	task, err := usefulwork.NewSumSquaresTask([]uint64{9, 10, 11})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := market.Create(task, "Alice", available-50, 777); err != nil {
		t.Fatal(err)
	}
	api := &apiServer{
		dataPath: dir, faucetEnabled: true,
		faucetWallet: "Alice", faucetAmount: 100,
		computeMarket: market,
	}
	_, status, err := api.dispenseFaucet(wallets["Bob"].Address)
	if status != http.StatusServiceUnavailable || err == nil {
		t.Fatalf("faucet spent reserved bounty: HTTP %d, %v", status, err)
	}
	saved, _, _, err := storage.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Blocks) != len(chain.Blocks) {
		t.Fatal("rejected faucet payout created a block")
	}
}

// The proof/receipt network context retains its existing PoUW domain; only
// its normal on-chain PRISM bounty payment changes to transaction V2.
func TestComputeSettlementEmitsNetworkBoundV2AndRecovers(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "legacy_network"
		if configured {
			name = "configured_network"
		}
		t.Run(name, func(t *testing.T) {
			chain, pos, wallets, err := createNode()
			if err != nil {
				t.Fatal(err)
			}
			if configured {
				chain.Config = blockchain.ChainConfig{
					ReservedAuthorities: reserved.AuthorityPolicy{
						Treasury: []string{wallets["Alice"].Address},
					},
				}
			}
			chainID, err := chain.ChainID()
			if err != nil {
				t.Fatal(err)
			}
			genesisHash := chain.Blocks[0].Hash
			dir := t.TempDir()
			if err := storage.Save(dir, chain, pos, wallets); err != nil {
				t.Fatal(err)
			}
			market := compute.NewMarketplace()
			task, err := usefulwork.NewSumSquaresTask([]uint64{31, 41, 59})
			if err != nil {
				t.Fatal(err)
			}
			job, err := market.Create(task, "Alice", 25, 810044)
			if err != nil {
				t.Fatal(err)
			}
			job, err = market.Claim(job.ID, wallets["Bob"].Address)
			if err != nil {
				t.Fatal(err)
			}
			proof, err := usefulwork.Execute(job.Task, wallets["Bob"])
			if err != nil {
				t.Fatal(err)
			}
			api := &apiServer{dataPath: dir, computeMarket: market}
			first, err := settleComputeJob(api, job.ID, proof)
			if err != nil {
				t.Fatal(err)
			}
			if first.Recovered {
				t.Fatal("unexpected first-call recovery")
			}
			saved, savedPoS, _, err := storage.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !saved.ValidateChain(savedPoS) {
				t.Fatal("chain invalid after v2 compute bounty")
			}
			found := false
			for _, block := range saved.Blocks {
				for _, tx := range block.Transactions {
					if tx.ID != first.SettlementTxID {
						continue
					}
					found = true
					if tx.Version != transaction.VersionV2 ||
						tx.ChainID != chainID || tx.GenesisHash != genesisHash ||
						tx.From != wallets["Alice"].Address ||
						tx.To != wallets["Bob"].Address || tx.Amount != 25 {
						t.Fatalf("compute payout has incorrect v2 fields: %+v", tx)
					}
					if err := saved.ValidateTransactionContext(tx); err != nil {
						t.Fatalf("compute payout rejected by chain: %v", err)
					}
				}
			}
			if !found {
				t.Fatal("compute settlement tx missing")
			}
			retry, err := settleComputeJob(api, job.ID, proof)
			if err != nil {
				t.Fatal(err)
			}
			if !retry.Recovered || retry.SettlementTxID != first.SettlementTxID {
				t.Fatalf("settlement recovery changed: %+v", retry)
			}
		})
	}
}
