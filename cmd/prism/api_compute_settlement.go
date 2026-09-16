package main

import (
	"fmt"

	"prism/internal/compute"
	"prism/internal/mempool"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
)

type apiComputeSettlementResult struct {
	Job            compute.Job
	Block          uint64
	SettlementTxID string
	BountyReward   uint64
	Recovered      bool
}

func settleComputeJob(
	api *apiServer,
	jobID string,
	proof usefulwork.Proof,
) (apiComputeSettlementResult, error) {

	if api == nil {
		return apiComputeSettlementResult{},
			fmt.Errorf("API server cannot be nil")
	}

	if api.computeMarket == nil {
		return apiComputeSettlementResult{},
			fmt.Errorf("compute marketplace is unavailable")
	}

	api.stateMu.Lock()
	defer api.stateMu.Unlock()

	job, err := api.computeMarket.Get(jobID)
	if err != nil {
		return apiComputeSettlementResult{}, err
	}

	switch job.Status {
	case compute.JobStatusClaimed:
		candidate := job

		if err := candidate.Complete(proof); err != nil {
			return apiComputeSettlementResult{}, err
		}

	case compute.JobStatusVerified:
		if job.ProofID != proof.ID {
			return apiComputeSettlementResult{},
				fmt.Errorf(
					"compute job already verified with another proof",
				)
		}

		if proof.Worker != job.Worker {
			return apiComputeSettlementResult{},
				fmt.Errorf(
					"proof worker does not own compute job",
				)
		}

		if proof.Task.ID != job.Task.ID {
			return apiComputeSettlementResult{},
				fmt.Errorf(
					"proof task does not match compute job",
				)
		}

		if err := usefulwork.VerifyProof(proof); err != nil {
			return apiComputeSettlementResult{}, err
		}

	default:
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"compute job cannot be settled from status %s",
				job.Status,
			)
	}

	chain, pos, wallets, err := api.loadState()
	if err != nil {
		return apiComputeSettlementResult{}, err
	}

	_, requesterWallet, err := resolveLocalWallet(
		job.Requester,
		wallets,
	)
	if err != nil {
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"compute requester must be a local wallet: %w",
				err,
			)
	}

	if requesterWallet.Address == job.Worker {
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"compute requester and worker cannot be identical",
			)
	}

	// Crash/retry recovery:
	//
	// If the blockchain block was saved successfully but persistence
	// of compute-jobs.json failed, recognize the exact proof +
	// requester->worker bounty transaction and finalize the marketplace
	// state without paying twice.
	for _, block := range chain.Blocks {
		proofFound := false

		for _, existingProof := range block.UsefulWork {
			if existingProof.ID != proof.ID {
				continue
			}

			if existingProof.Worker != job.Worker {
				return apiComputeSettlementResult{},
					fmt.Errorf(
						"on-chain proof worker does not match compute job",
					)
			}

			if existingProof.Task.ID != job.Task.ID {
				return apiComputeSettlementResult{},
					fmt.Errorf(
						"on-chain proof task does not match compute job",
					)
			}

			proofFound = true
			break
		}

		if !proofFound {
			continue
		}

		for _, tx := range block.Transactions {
			if tx.From != requesterWallet.Address {
				continue
			}

			if tx.To != job.Worker {
				continue
			}

			if tx.Amount != job.Reward {
				continue
			}

			completedJob := job

			if job.Status != compute.JobStatusVerified {
				completedJob, err =
					api.computeMarket.Complete(
						jobID,
						proof,
					)

				if err != nil {
					return apiComputeSettlementResult{},
						fmt.Errorf(
							"cannot recover compute marketplace completion: %w",
							err,
						)
				}
			}

			return apiComputeSettlementResult{
				Job:            completedJob,
				Block:          block.Height,
				SettlementTxID: tx.ID,
				BountyReward:   job.Reward,
				Recovered:      true,
			}, nil
		}

		return apiComputeSettlementResult{},
			fmt.Errorf(
				"proof %s is already on-chain without matching compute bounty transaction",
				proof.ID,
			)
	}

	if job.Status == compute.JobStatusVerified {
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"compute job is verified but its settlement transaction is missing",
			)
	}

	availableBalance, err :=
		chain.AvailableBalanceOf(
			requesterWallet.Address,
		)

	if err != nil {
		return apiComputeSettlementResult{}, err
	}

	if availableBalance < job.Reward {
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"insufficient requester balance for compute bounty: have %d PRISM, need %d PRISM",
				availableBalance,
				job.Reward,
			)
	}

	pool := mempool.New()

	nonce, err := pool.NextNonce(
		requesterWallet.Address,
		chain,
	)
	if err != nil {
		return apiComputeSettlementResult{}, err
	}

	tx := transaction.New(
		requesterWallet.Address,
		job.Worker,
		job.Reward,
		nonce,
		requesterWallet.PublicKeyHex(),
	)

	if err := tx.Sign(
		requesterWallet.PrivateKey,
	); err != nil {
		return apiComputeSettlementResult{}, err
	}

	if err := pool.Add(
		tx,
		chain,
	); err != nil {
		return apiComputeSettlementResult{}, err
	}

	if len(chain.Blocks) == 0 {
		return apiComputeSettlementResult{},
			fmt.Errorf("blockchain is empty")
	}

	lastBlock :=
		chain.Blocks[len(chain.Blocks)-1]

	proposer, err := pos.SelectProposer(
		lastBlock.Hash,
		lastBlock.Height+1,
	)
	if err != nil {
		return apiComputeSettlementResult{}, err
	}

	// One atomic Prism block:
	//
	// 1. requester pays the marketplace bounty;
	// 2. worker proof is committed on-chain;
	// 3. normal PoUW consensus reward is applied independently.
	block, err := chain.AddBlock(
		pool.Transactions(),
		[]usefulwork.Proof{
			proof,
		},
		proposer.Address,
		pos,
	)
	if err != nil {
		return apiComputeSettlementResult{}, err
	}

	if err := api.saveState(
		chain,
		pos,
		wallets,
	); err != nil {
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"cannot persist compute settlement block: %w",
				err,
			)
	}

	completedJob, err :=
		api.computeMarket.Complete(
			jobID,
			proof,
		)

	if err != nil {
		return apiComputeSettlementResult{},
			fmt.Errorf(
				"compute settlement is on-chain in block %d but marketplace completion failed: %w",
				block.Height,
				err,
			)
	}

	return apiComputeSettlementResult{
		Job:            completedJob,
		Block:          block.Height,
		SettlementTxID: tx.ID,
		BountyReward:   job.Reward,
		Recovered:      false,
	}, nil
}
