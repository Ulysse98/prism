package blockchain

import (
	"fmt"
	"math"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

// ValidateBootstrapCandidateV061 performs a READ-ONLY preflight against the
// current canonical chain and its actual PoS validator set. It neither appends
// a block nor activates authorities. Accepting a bootstrap in consensus still
// requires a separate protocol change and deterministic replay integration.
func (bc *Blockchain) ValidateBootstrapCandidateV061(
	bootstrap reserved.GovernanceBootstrap,
	pos *consensus.ProofOfStake,
) error {
	if bc == nil || pos == nil {
		return fmt.Errorf("bootstrap candidate needs a chain and PoS state")
	}
	if len(bc.Blocks) == 0 {
		return fmt.Errorf("cannot bootstrap an empty chain")
	}
	if !bc.ValidateChain(pos) {
		return fmt.Errorf("cannot bootstrap an invalid canonical chain")
	}

	last := bc.Blocks[len(bc.Blocks)-1]
	if last.Height == math.MaxUint64 {
		return fmt.Errorf("bootstrap height overflow")
	}
	nextHeight := last.Height + 1

	chainID, err := bc.ChainID()
	if err != nil {
		return fmt.Errorf("bootstrap chain identity: %w", err)
	}
	if bootstrap.ChainID != chainID {
		return fmt.Errorf("bootstrap targets a different chain ID")
	}
	if bootstrap.GenesisHash != bc.Blocks[0].Hash {
		return fmt.Errorf("bootstrap targets a different genesis")
	}
	if bootstrap.ActivationHeight != nextHeight {
		return fmt.Errorf("bootstrap activation must match next block height: expected %d", nextHeight)
	}

	governance, err := bc.GetGovernanceState()
	if err != nil {
		return fmt.Errorf("cannot replay current governance: %w", err)
	}

	// ValidateChain ensured every local PoS validator and locked stake match.
	// We pass this verified snapshot rather than caller-defined vote weights.
	validators := append([]consensus.Validator(nil), pos.Validators...)
	if err := governance.ApplyBootstrapV061(
		bootstrap,
		validators,
		bc.Blocks[0].Hash,
		nextHeight,
	); err != nil {
		return fmt.Errorf("bootstrap candidate denied: %w", err)
	}
	// GovernanceState is transient replay state, not a mutation of bc.Config.
	return nil
}
