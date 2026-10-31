//go:build linux

package finality

// Experimental validated PREVOTE-only bridge; it does not implement BFT finality.
// It pins an authenticated parent at explicit initialization and replays full
// parent+candidate validation before an atomic PREVOTE transition can be stored.
// Unprotected legacy gates cannot open the new binding namespace.
import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

const validatedPrevoteModeV065 = "prism-validated-prevote-v1"

// InitPrismValidatedPrevoteGateV065 requires the caller to authenticate its
// canonical parent and snapshot before this call and keep them stable during copying.
// The expectedParentHash is an externally authenticated tip, not a derived trust root.
// Parent remains pinned at this height; this lab DOES NOT track live reorgs.
func InitPrismValidatedPrevoteGateV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey, parent *blockchain.Blockchain, expectedParentHash string) error {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return err
	}
	if !validHash(expectedParentHash) || parent == nil {
		return errors.New("missing authenticated parent")
	}
	b.Mode, b.ParentHash = validatedPrevoteModeV065, expectedParentHash
	data, err := json.Marshal(parent)
	if err != nil {
		return err
	}
	var frozen blockchain.Blockchain
	if err := json.Unmarshal(data, &frozen); err != nil {
		return err
	}
	if len(frozen.Blocks) == 0 || frozen.Blocks[0].Hash != genesisHash {
		return errors.New("wrong parent genesis")
	}
	tip := frozen.Blocks[len(frozen.Blocks)-1]
	if tip.Hash != expectedParentHash || tip.Height == math.MaxUint64 || tip.Height+1 != height {
		return errors.New("wrong parent tip or height")
	}
	pos := &consensus.ProofOfStake{Validators: append([]consensus.Validator(nil), validators...)}
	if !frozen.ValidateChain(pos) {
		return errors.New("invalid pinned parent chain")
	}
	return initDurableWithParentV065(dir, b, &frozen)
}

// verifyValidatedPrevoteHistoryV065 runs inside the gate's file lock during
// record replay; it MUST validate the persisted witnesses, not the original bool.
func verifyValidatedPrevoteHistoryV065(r durableBFTRecordV065, b durableBindingV065, validators []consensus.Validator) error {
	if r.Parent == nil || len(r.Parent.Blocks) == 0 {
		return errors.New("pinned parent missing")
	}
	tip := r.Parent.Blocks[len(r.Parent.Blocks)-1]
	if tip.Hash != b.ParentHash || r.Parent.Blocks[0].Hash != b.GenesisHash || tip.Height == math.MaxUint64 || tip.Height+1 != b.Height {
		return errors.New("pinned parent binding mismatch")
	}
	pos := &consensus.ProofOfStake{Validators: append([]consensus.Validator(nil), validators...)}
	if !r.Parent.ValidateChain(pos) {
		return errors.New("pinned parent no longer validates")
	}
	for _, e := range r.Events {
		// PREVOTE-only scope forbids a PRECOMMIT/round advance/finalization.
		if e.Kind != durableGatePrevoteV065 || e.Candidate == nil || !e.BlockValidated || e.Proposal == "" || e.Evidence != nil || e.NextRound != 0 {
			return errors.New("unsupported or unproven transition in validated prevote mode")
		}
		ctx := VoteContext{ChainID: b.ChainID, GenesisHash: b.GenesisHash, Height: b.Height, Round: 0, Step: stepPrevote, BlockHash: e.Proposal, ValidatorSetHash: b.ValidatorSetHash}
		if err := ValidatePrismProposalV065(r.Parent, pos, *e.Candidate, ctx, b.ChainID, b.ParentHash); err != nil {
			return fmt.Errorf("candidate witness: %w", err)
		}
	}
	return nil
}

// SignPrismValidatedPrevoteV065 accepts NO blockValidated bool and no raw
// caller-managed parent. It persists a complete candidate witness and checks
// that candidate again when replaying the record, including on retry/restart.
// This deliberately supports one round-0 non-NIL PREVOTE only.
func SignPrismValidatedPrevoteV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey, candidate blockchain.Block, expectedParentHash string) (SignedVote, error) {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return SignedVote{}, err
	}
	if !validHash(expectedParentHash) {
		return SignedVote{}, errors.New("invalid expected parent")
	}
	b.Mode, b.ParentHash = validatedPrevoteModeV065, expectedParentHash
	// Deep-copy before entering the durable critical section.
	data, err := json.Marshal(candidate)
	if err != nil {
		return SignedVote{}, err
	}
	var frozen blockchain.Block
	if err := json.Unmarshal(data, &frozen); err != nil {
		return SignedVote{}, err
	}
	if !validHash(frozen.Hash) {
		return SignedVote{}, errors.New("noncanonical candidate hash")
	}
	event := durableGateEventV065{Kind: durableGatePrevoteV065, Proposal: frozen.Hash, BlockValidated: true, Candidate: &frozen}
	return durableGateApplyHookV065(dir, b, validators, key, event, nil)
}
