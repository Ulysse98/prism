package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"prism/internal/consensus"
	"prism/internal/escrow"
)

// The v0.65 hash domain covers every Block field, including escrow fundings.
// Blocks without escrow fundings retain their exact historical hash functions.
func calculateEscrowBlockHashV065(block Block) string {
	block.Hash = ""
	payload, err := json.Marshal(block)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(append([]byte("prism-escrow-block-v065|"), payload...))
	return hex.EncodeToString(sum[:])
}

// v0.65 limits each funding block to exactly one independent signed intent.
// No transaction, proof, governance operation, or other intent can share it.
func standaloneEscrowFundingBlockV065(block Block) bool {
	return len(block.EscrowFundings) == 1 &&
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

type escrowFundingIndexV065 struct {
	locked  map[string]uint64
	jobIDs  map[string]bool
	intents map[string]bool
	nonces  map[string]bool
}

func newEscrowFundingIndexV065() *escrowFundingIndexV065 {
	return &escrowFundingIndexV065{locked: map[string]uint64{}, jobIDs: map[string]bool{}, intents: map[string]bool{}, nonces: map[string]bool{}}
}

func (i *escrowFundingIndexV065) apply(a escrow.FundingAuthorization, blockHeight uint64, chainID, genesisHash string) error {
	if err := escrow.VerifyFunding(a); err != nil {
		return fmt.Errorf("invalid signed escrow funding: %w", err)
	}
	f := a.Funding
	if f.ChainID != chainID || f.GenesisHash != genesisHash || f.FundedAtHeight != blockHeight {
		return fmt.Errorf("escrow funding consensus context mismatch")
	}
	if i.jobIDs[f.JobID] || i.intents[f.FundingTxID] || i.nonces[fmt.Sprintf("%s/%d", f.Requester, a.Nonce)] {
		return fmt.Errorf("duplicate escrow job, funding ID, or requester nonce")
	}
	if i.locked[f.Requester] > math.MaxUint64-f.Amount {
		return fmt.Errorf("escrow lock overflow")
	}
	i.locked[f.Requester] += f.Amount
	i.jobIDs[f.JobID] = true
	i.intents[f.FundingTxID] = true
	i.nonces[fmt.Sprintf("%s/%d", f.Requester, a.Nonce)] = true
	return nil
}

// GetEscrowLockedBalancesV065 reconstructs the immutable funding locks from
// canonical blocks. This is not SQLite state. It supports FUND only: no coins
// can leave escrow until authenticated RELEASE/REFUND transitions are added.
func (bc *Blockchain) GetEscrowLockedBalancesV065() (map[string]uint64, error) {
	if bc == nil || len(bc.Blocks) == 0 {
		return nil, fmt.Errorf("missing escrow blockchain")
	}
	idx := newEscrowFundingIndexV065()
	var chainID string
	for j, block := range bc.Blocks {
		if len(block.EscrowFundings) == 0 {
			continue
		}
		if j == 0 || !standaloneEscrowFundingBlockV065(block) {
			return nil, fmt.Errorf("invalid escrow funding block at height %d", block.Height)
		}
		if chainID == "" {
			var err error
			chainID, err = bc.ChainID()
			if err != nil {
				return nil, err
			}
		}
		if err := idx.apply(block.EscrowFundings[0], block.Height, chainID, bc.Blocks[0].Hash); err != nil {
			return nil, err
		}
	}
	return idx.locked, nil
}

// AddEscrowFundingBlockV065 appends a canonical FUND-only block after full
// candidate-chain validation. Experimental isolated-testnet entrypoint only.
// It does not expose a public API and cannot release or refund any funds.
func (bc *Blockchain) AddEscrowFundingBlockV065(auth escrow.FundingAuthorization, proposer string, pos *consensus.ProofOfStake) (Block, error) {
	if bc == nil || len(bc.Blocks) == 0 || pos == nil {
		return Block{}, fmt.Errorf("missing blockchain or PoS")
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
	reward, err := consensus.BoundedPoolReward(consensus.DefaultRewardPolicy().ProposerReward, emission.ProposerEmission, consensus.DefaultSupplyPolicy().ProposerRewardPool)
	if err != nil {
		return Block{}, err
	}
	block := Block{Height: nextHeight, Timestamp: time.Now().UTC(), PreviousHash: tip.Hash, Proposer: proposer, Reward: reward, EscrowFundings: []escrow.FundingAuthorization{auth}}
	block.Hash = CalculateHash(block)
	if err := bc.AppendValidatedBlock(block, pos); err != nil {
		return Block{}, fmt.Errorf("escrow funding block rejected: %w", err)
	}
	return block, nil
}
