package finality

import (
	"errors"
	"fmt"
	"math"

	"prism/internal/consensus"
)

// IsolatedSignerRequestV068 is ONLY a proposed non-signing IPC contract.
// It carries no private key and is not a proof that the attached context was
// authenticated, serialized, or checked inside an isolated key-owning process.
type IsolatedSignerRequestV068 struct {
	Vote          VoteContext `json:"vote"`
	SignerAddress string      `json:"signer_address"`
	ParentHash    string      `json:"parent_hash"`
}

// CheckIsolatedSignerRequestV068 is a pure, non-signing admission preflight
// against caller-provisioned trust inputs. The floor and witness MUST be
// independently authenticated and anchored, and the parent/block state must
// be independently validated by a future exclusive signer boundary.
//
// Phase 2A.1 is intentionally limited to one non-NIL round-0 PREVOTE at the
// height immediately after the exact pinned parent. It makes no signature,
// stores no slot, acquires no lock and does NOT prevent double-signing.
func CheckIsolatedSignerRequestV068(
	req IsolatedSignerRequestV068,
	trustedFloor TrustedCheckpointFloorV068,
	trustedWitness TrustedCheckpointFloorWitnessV068,
	trustedValidators []consensus.Validator,
) error {
	floorDigest, err := CheckpointFloorDigestV068(trustedFloor)
	if err != nil {
		return fmt.Errorf("invalid externally trusted floor: %w", err)
	}
	if trustedWitness.Revision != trustedFloor.Revision || trustedWitness.Digest != floorDigest {
		return errors.New("trusted external witness differs from floor")
	}

	anchor := trustedFloor.Anchor
	if anchor.ParentHeight == math.MaxUint64 {
		return errors.New("parent height overflows next signing height")
	}
	if err := validateContext(req.Vote); err != nil {
		return fmt.Errorf("invalid vote context: %w", err)
	}
	if req.Vote.Step != stepPrevote || req.Vote.Round != 0 || req.Vote.Nil {
		return errors.New("2A.1 contract supports only non-NIL round-0 PREVOTE")
	}
	if req.Vote.ChainID != anchor.ChainID ||
		req.Vote.GenesisHash != anchor.GenesisHash ||
		req.Vote.Height != anchor.ParentHeight+1 ||
		req.ParentHash != anchor.ParentHash ||
		req.Vote.ValidatorSetHash != anchor.ValidatorSetHash {
		return errors.New("request is not bound to exact trusted parent, height or validator set")
	}
	snapshotHash, err := ValidatorSetHash(trustedValidators)
	if err != nil {
		return fmt.Errorf("invalid trusted validator snapshot: %w", err)
	}
	if snapshotHash != anchor.ValidatorSetHash {
		return errors.New("validator snapshot differs from exact trusted floor")
	}
	found := false
	for _, validator := range trustedValidators {
		if validator.Address == req.SignerAddress && validator.Stake > 0 {
			found = true
			break
		}
	}
	if !found {
		return errors.New("requested signer is not a trusted staked validator")
	}
	return nil
}
