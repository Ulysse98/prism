package blockchain

import (
	"fmt"
	"math"
	"time"

	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/escrow"
	"prism/internal/usefulwork"
)

// EscrowRefundV065 is a permissionless expiry operation. Its only target is
// the canonical job; the beneficiary is fixed by its signed FUND record.
type EscrowRefundV065 struct {
	JobID string `json:"job_id"`
}

// A v0.65 escrow block carries precisely one escrow operation. It cannot
// combine a state transition with ordinary transactions or reward-bearing work.
func escrowOperationCountV065(block Block) int {
	return len(block.EscrowFundings) + len(block.EscrowClaims) +
		len(block.EscrowReleases) + len(block.EscrowRefunds)
}

func standaloneEscrowBlockV065(block Block) bool {
	return escrowOperationCountV065(block) == 1 &&
		len(block.Transactions) == 0 &&
		len(block.UsefulWork) == 0 &&
		len(block.Humanity) == 0 &&
		len(block.ParticipationClaims) == 0 &&
		len(block.ReservedAuthorizations) == 0 &&
		len(block.ReservedGrants) == 0 &&
		len(block.ReservedRevocations) == 0 &&
		len(block.AuthorityChanges) == 0 &&
		len(block.AuthorityProposals) == 0 &&
		len(block.AuthorityExecutions) == 0 &&
		len(block.ReservedTransferProposals) == 0 &&
		len(block.ReservedTransferExecutions) == 0 &&
		len(block.GovernanceBootstraps) == 0
}

func (idx *escrowFundingIndexV065) applyClaimV065(
	claim compute.ClaimAuthorization, height uint64,
) error {
	current, exists := idx.records[claim.JobID]
	if !exists {
		return fmt.Errorf("escrow claim references unknown job")
	}
	next, err := escrow.VerifyEscrowClaimV065(current, claim, height)
	if err != nil {
		return fmt.Errorf("escrow claim rejected: %w", err)
	}
	idx.records[claim.JobID] = next
	return nil
}

func (idx *escrowFundingIndexV065) applyReleaseV065(
	proof usefulwork.Proof, height uint64, state *State,
) error {
	current, exists := idx.records[proof.JobID]
	if !exists {
		return fmt.Errorf("escrow release references unknown job")
	}
	next, err := escrow.VerifyEscrowReleaseV065(current, proof, height)
	if err != nil {
		return fmt.Errorf("escrow release rejected: %w", err)
	}
	requester := current.Funding.Requester
	worker := current.Worker
	amount := current.Funding.Amount
	if idx.locked[requester] < amount {
		return fmt.Errorf("escrow release lock underflow")
	}
	// The release is an atomic debit/credit of existing tokens. A replay used
	// solely to reconstruct escrow locks supplies nil state; GetState replay
	// always supplies the actual ledger balances and verifies conservation.
	if state != nil {
		if state.Balances[requester] < amount {
			return fmt.Errorf("escrow release exceeds requester balance")
		}
		if state.Balances[worker] > math.MaxUint64-amount {
			return fmt.Errorf("escrow release worker balance overflow")
		}
		state.Balances[requester] -= amount
		state.Balances[worker] += amount
	}
	idx.locked[requester] -= amount
	idx.records[proof.JobID] = next
	return nil
}

func (idx *escrowFundingIndexV065) applyRefundV065(
	intent EscrowRefundV065, height uint64,
) error {
	current, exists := idx.records[intent.JobID]
	if !exists {
		return fmt.Errorf("escrow refund references unknown job")
	}
	next, err := escrow.VerifyEscrowRefundV065(current, height)
	if err != nil {
		return fmt.Errorf("escrow refund rejected: %w", err)
	}
	requester := current.Funding.Requester
	amount := current.Funding.Amount
	if idx.locked[requester] < amount {
		return fmt.Errorf("escrow refund lock underflow")
	}
	// Refund changes only spendability: FUND never debited the gross balance.
	idx.locked[requester] -= amount
	idx.records[intent.JobID] = next
	return nil
}

// replayEscrowIndexV065 derives escrow state from consensus block operations.
// It does not independently account for ordinary transactions: GetState is
// responsible for the gross-balance and supply checks.
func (bc *Blockchain) replayEscrowIndexV065() (*escrowFundingIndexV065, error) {
	if bc == nil || len(bc.Blocks) == 0 {
		return nil, fmt.Errorf("missing escrow blockchain")
	}
	idx := newEscrowFundingIndexV065()
	chainID, err := bc.ChainID()
	if err != nil {
		return nil, err
	}
	for i, block := range bc.Blocks {
		if escrowOperationCountV065(block) == 0 {
			continue
		}
		if i == 0 || !standaloneEscrowBlockV065(block) {
			return nil, fmt.Errorf("invalid escrow block at height %d", block.Height)
		}
		switch {
		case len(block.EscrowFundings) == 1:
			if err := idx.apply(block.EscrowFundings[0], block.Height, chainID, bc.Blocks[0].Hash); err != nil {
				return nil, err
			}
		case len(block.EscrowClaims) == 1:
			if err := idx.applyClaimV065(block.EscrowClaims[0], block.Height); err != nil {
				return nil, err
			}
		case len(block.EscrowReleases) == 1:
			if err := idx.applyReleaseV065(block.EscrowReleases[0], block.Height, nil); err != nil {
				return nil, err
			}
		case len(block.EscrowRefunds) == 1:
			if err := idx.applyRefundV065(block.EscrowRefunds[0], block.Height); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid escrow operation")
		}
	}
	return idx, nil
}

// GetEscrowRecordV065 returns the block-replayed state of one job, not an
// SQLite projection. Call ValidateChain before trusting it for external use.
func (bc *Blockchain) GetEscrowRecordV065(jobID string) (escrow.Escrow, error) {
	idx, err := bc.replayEscrowIndexV065()
	if err != nil {
		return escrow.Escrow{}, err
	}
	value, exists := idx.records[jobID]
	if !exists {
		return escrow.Escrow{}, fmt.Errorf("escrow job not found")
	}
	return value, nil
}

// addEscrowOperationBlockV065 is experimental, for isolated testnets only.
// It performs full candidate-chain validation before accepting an operation.
func (bc *Blockchain) addEscrowOperationBlockV065(
	payload Block, proposer string, pos *consensus.ProofOfStake,
) (Block, error) {
	if bc == nil || len(bc.Blocks) == 0 || pos == nil {
		return Block{}, fmt.Errorf("missing blockchain or PoS")
	}
	if !standaloneEscrowBlockV065(payload) {
		return Block{}, fmt.Errorf("escrow operation must stand alone")
	}
	if err := bc.ValidateValidatorSet(pos); err != nil {
		return Block{}, err
	}
	tip := bc.Blocks[len(bc.Blocks)-1]
	nextHeight := tip.Height + 1
	selection, err := pos.SelectProposer(tip.Hash, nextHeight)
	if err != nil {
		return Block{}, err
	}
	if proposer != selection.Address {
		return Block{}, fmt.Errorf("unexpected escrow proposer")
	}
	emission, err := bc.GetEmissionState()
	if err != nil {
		return Block{}, err
	}
	reward, err := consensus.BoundedPoolReward(
		consensus.DefaultRewardPolicy().ProposerReward,
		emission.ProposerEmission,
		consensus.DefaultSupplyPolicy().ProposerRewardPool,
	)
	if err != nil {
		return Block{}, err
	}
	payload.Height = nextHeight
	payload.Timestamp = time.Now().UTC()
	payload.PreviousHash = tip.Hash
	payload.Proposer = proposer
	payload.Reward = reward
	payload.Hash = CalculateHash(payload)
	if err := bc.AppendValidatedBlock(payload, pos); err != nil {
		return Block{}, fmt.Errorf("escrow operation rejected: %w", err)
	}
	return payload, nil
}

func (bc *Blockchain) AddEscrowClaimBlockV065(
	claim compute.ClaimAuthorization, proposer string, pos *consensus.ProofOfStake,
) (Block, error) {
	return bc.addEscrowOperationBlockV065(Block{
		EscrowClaims: []compute.ClaimAuthorization{claim},
	}, proposer, pos)
}

func (bc *Blockchain) AddEscrowReleaseBlockV065(
	proof usefulwork.Proof, proposer string, pos *consensus.ProofOfStake,
) (Block, error) {
	return bc.addEscrowOperationBlockV065(Block{
		EscrowReleases: []usefulwork.Proof{proof},
	}, proposer, pos)
}

func (bc *Blockchain) AddEscrowRefundBlockV065(
	jobID string, proposer string, pos *consensus.ProofOfStake,
) (Block, error) {
	return bc.addEscrowOperationBlockV065(Block{
		EscrowRefunds: []EscrowRefundV065{{JobID: jobID}},
	}, proposer, pos)
}
