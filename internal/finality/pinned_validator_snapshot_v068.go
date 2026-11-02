package finality

import (
	"encoding/json"
	"errors"
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// PinnedValidatorAnchorV068 is an EXTERNALLY AUTHENTICATED trust input.
// This lab cannot establish an anchor's authority, chain finality, epoch,
// historical validator set, or protection against host/storage rollback.
// The caller MUST obtain and validate this pin independently of untrusted
// candidate chain or peer data. Never build it from the data being checked
// in a production trust decision.
type PinnedValidatorAnchorV068 struct {
	ChainID          string
	GenesisHash      string
	ParentHeight     uint64
	ParentHash       string
	ValidatorSetHash string
}

// VerifyPinnedValidatorSnapshotV068 checks a candidate parent chain and
// validator snapshot against an independently trusted pin. It signs nothing,
// emits no vote, changes no blockchain state, and does not grant finality.
// It only checks the snapshot against the parent's CURRENT locked-stake
// state; historical height-to-epoch derivation remains unimplemented.
func VerifyPinnedValidatorSnapshotV068(
	parent *blockchain.Blockchain,
	validators []consensus.Validator,
	pin PinnedValidatorAnchorV068,
) error {
	if parent == nil || len(parent.Blocks) == 0 {
		return errors.New("missing candidate parent chain")
	}
	if pin.ChainID == "" || !validHash(pin.GenesisHash) ||
		!validHash(pin.ParentHash) || !validHash(pin.ValidatorSetHash) {
		return errors.New("missing or noncanonical external trust anchor")
	}

	// Detach caller data before validating it. Caller must keep input stable
	// while copying; this is not an atomic live node snapshot acquisition.
	encoded, err := json.Marshal(parent)
	if err != nil {
		return fmt.Errorf("encode candidate parent: %w", err)
	}
	var candidate blockchain.Blockchain
	if err := json.Unmarshal(encoded, &candidate); err != nil {
		return fmt.Errorf("decode candidate parent: %w", err)
	}
	frozen := append([]consensus.Validator(nil), validators...)
	setHash, err := ValidatorSetHash(frozen)
	if err != nil {
		return fmt.Errorf("invalid candidate validator set: %w", err)
	}
	if setHash != pin.ValidatorSetHash {
		return errors.New("candidate validator set differs from trusted pin")
	}
	if len(candidate.Blocks) == 0 || candidate.Blocks[0].Hash != pin.GenesisHash {
		return errors.New("parent genesis differs from trusted pin")
	}
	tip := candidate.Blocks[len(candidate.Blocks)-1]
	if tip.Height != pin.ParentHeight || tip.Hash != pin.ParentHash {
		return errors.New("parent height or tip differs from trusted pin")
	}
	chainID, err := candidate.ChainID()
	if err != nil {
		return fmt.Errorf("derive candidate chain identity: %w", err)
	}
	if chainID != pin.ChainID {
		return errors.New("candidate chain identity differs from trusted pin")
	}
	pos := &consensus.ProofOfStake{Validators: frozen}
	if !candidate.ValidateChain(pos) {
		return errors.New("candidate parent chain or current locked stakes failed validation")
	}
	return nil
}
