package finality

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// ValidatePrismProposalV065 is a NON-SIGNING experimental candidate validator.
// It binds a non-NIL vote context to a copy of a trusted parent chain, verifies
// the parent and its PoS snapshot, and validates the exact proposed block with
// PRISM's ordinary AppendValidatedBlock/ValidateChain rules.
//
// The caller MUST authenticate the chain ID, expected parent tip, and validator
// snapshot independently. The provided objects must not be mutated concurrently.
// A successful return MUST NOT be treated as authorization by the signing gate:
// candidate state can become stale before signing, and replay of the durable
// gate still trusts BlockValidated. The eventual gate must recheck all of these
// facts under its serialization boundary and recover their provenance.
func ValidatePrismProposalV065(
	parent *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	proposed blockchain.Block,
	vote VoteContext,
	trustedChainID, expectedParentHash string,
) error {
	if parent == nil || pos == nil || len(parent.Blocks) == 0 {
		return errors.New("missing parent chain or validator snapshot")
	}
	if err := validateContext(vote); err != nil {
		return fmt.Errorf("invalid vote context: %w", err)
	}
	if vote.Nil {
		return errors.New("NIL vote has no candidate block to validate")
	}
	if trustedChainID == "" || vote.ChainID != trustedChainID {
		return errors.New("untrusted vote chain ID")
	}
	if !validHash(expectedParentHash) {
		return errors.New("invalid expected parent hash")
	}

	// JSON round-trips detach slice/map fields in parent and proposed block.
	// The caller must still ensure these objects are stable while copying.
	parentJSON, err := json.Marshal(parent)
	if err != nil {
		return fmt.Errorf("encode parent: %w", err)
	}
	var detached blockchain.Blockchain
	if err := json.Unmarshal(parentJSON, &detached); err != nil {
		return fmt.Errorf("decode parent: %w", err)
	}
	candidateJSON, err := json.Marshal(proposed)
	if err != nil {
		return fmt.Errorf("encode candidate: %w", err)
	}
	var candidate blockchain.Block
	if err := json.Unmarshal(candidateJSON, &candidate); err != nil {
		return fmt.Errorf("decode candidate: %w", err)
	}
	frozenPOS := &consensus.ProofOfStake{
		Validators: append([]consensus.Validator(nil), pos.Validators...),
	}
	setHash, err := ValidatorSetHash(frozenPOS.Validators)
	if err != nil {
		return fmt.Errorf("invalid validator snapshot: %w", err)
	}
	if setHash != vote.ValidatorSetHash {
		return errors.New("vote validator-set hash mismatch")
	}
	if len(detached.Blocks) == 0 || vote.GenesisHash != detached.Blocks[0].Hash {
		return errors.New("vote genesis hash mismatch")
	}
	last := detached.Blocks[len(detached.Blocks)-1]
	if last.Hash != expectedParentHash {
		return errors.New("parent changed or does not match trusted tip")
	}
	if last.Height == math.MaxUint64 || candidate.Height != last.Height+1 || candidate.Height != vote.Height {
		return errors.New("candidate height does not extend trusted tip")
	}
	if candidate.PreviousHash != last.Hash {
		return errors.New("candidate does not extend trusted parent hash")
	}
	if !validHash(candidate.Hash) || candidate.Hash != vote.BlockHash {
		return errors.New("vote block hash mismatch")
	}
	if !detached.ValidateChain(frozenPOS) {
		return errors.New("parent chain failed full validation")
	}
	if err := detached.AppendValidatedBlock(candidate, frozenPOS); err != nil {
		return fmt.Errorf("candidate failed full chain validation: %w", err)
	}
	return nil
}
