package finality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// TrustedCheckpointFloorV068 is an operator-provisioned, independently
// authenticated historical checkpoint floor. Merely computing its digest does
// NOT authenticate the floor, nor does holding it in memory prevent rollback.
type TrustedCheckpointFloorV068 struct {
	Revision uint64
	Anchor   PinnedValidatorAnchorV068
}

// TrustedCheckpointFloorWitnessV068 is expected from an independent trust
// source. A caller-supplied or peer-supplied digest is NOT an anti-rollback root.
// The caller MUST authenticate and monotonically retain this exact witness
// outside the candidate chain and any rollbackable local state.
type TrustedCheckpointFloorWitnessV068 struct {
	Revision uint64
	Digest   string
}

const checkpointFloorDomainV068 = "prism-finality-v068/checkpoint-floor/v1\x00"

// CheckpointFloorDigestV068 yields a canonical commitment to the entire floor.
// Hashing is NOT a signature, persistence, independent authentication, or BFT.
func CheckpointFloorDigestV068(f TrustedCheckpointFloorV068) (string, error) {
	a := f.Anchor
	if f.Revision == 0 || a.ChainID == "" || !validHash(a.GenesisHash) ||
		!validHash(a.ParentHash) || !validHash(a.ValidatorSetHash) {
		return "", errors.New("invalid trusted floor record")
	}
	wire := struct {
		Revision         uint64 `json:"revision"`
		ChainID          string `json:"chain_id"`
		GenesisHash      string `json:"genesis_hash"`
		ParentHeight     uint64 `json:"parent_height"`
		ParentHash       string `json:"parent_hash"`
		ValidatorSetHash string `json:"validator_set_hash"`
	}{
		Revision: f.Revision,
		ChainID:  a.ChainID, GenesisHash: a.GenesisHash,
		ParentHeight: a.ParentHeight, ParentHash: a.ParentHash,
		ValidatorSetHash: a.ValidatorSetHash,
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return "", fmt.Errorf("encode trust floor: %w", err)
	}
	digest := sha256.Sum256(append([]byte(checkpointFloorDomainV068), data...))
	return hex.EncodeToString(digest[:]), nil
}

// VerifyAndAdvanceCheckpointFloorV068 verifies a signed checkpoint with the
// existing 1B.2 verifier, then makes a monotonic, non-mutating floor decision.
// It assumes 'current' and 'witness' were independently authenticated and
// kept outside rollbackable peer/local state. They MUST NOT be constructed
// from the candidate being checked. This function DOES NOT persist outputs.
// For a higher height, the caller must durably and atomically anchor the new
// witness elsewhere before relying on it; that lifecycle is not provided.
func VerifyAndAdvanceCheckpointFloorV068(
	parent *blockchain.Blockchain,
	validators []consensus.Validator,
	signed SignedValidatorCheckpointV068,
	policy CheckpointTrustPolicyV068,
	current TrustedCheckpointFloorV068,
	witness TrustedCheckpointFloorWitnessV068,
) (TrustedCheckpointFloorV068, TrustedCheckpointFloorWitnessV068, error) {
	reject := func(err error) (TrustedCheckpointFloorV068, TrustedCheckpointFloorWitnessV068, error) {
		return TrustedCheckpointFloorV068{}, TrustedCheckpointFloorWitnessV068{}, err
	}
	digest, err := CheckpointFloorDigestV068(current)
	if err != nil {
		return reject(err)
	}
	if witness.Revision != current.Revision || witness.Digest != digest ||
		!validHash(witness.Digest) {
		return reject(errors.New("external floor witness differs from locally supplied floor"))
	}
	old := current.Anchor
	if old.ChainID != policy.ChainID || old.GenesisHash != policy.GenesisHash {
		return reject(errors.New("floor is outside independently trusted network"))
	}
	next := signed.Anchor
	if next.ChainID != old.ChainID || next.GenesisHash != old.GenesisHash {
		return reject(errors.New("signed checkpoint disagrees with trusted floor network"))
	}
	if next.ParentHeight < old.ParentHeight {
		return reject(errors.New("checkpoint replay below independently trusted floor"))
	}
	if next.ParentHeight == old.ParentHeight && next != old {
		return reject(errors.New("conflicting checkpoint at immutable trusted height"))
	}
	if next.ParentHeight > old.ParentHeight && current.Revision == math.MaxUint64 {
		return reject(errors.New("trusted floor revision overflow"))
	}
	// Verify signature, policy revocation/height window, exact parent-chain
	// context, and current stakes even for an equal-height idempotent retry.
	if err := VerifySignedPinnedValidatorSnapshotV068(parent, validators, signed, policy); err != nil {
		return reject(fmt.Errorf("invalid checkpoint: %w", err))
	}
	if next.ParentHeight == old.ParentHeight {
		return current, witness, nil
	}
	updated := TrustedCheckpointFloorV068{Revision: current.Revision + 1, Anchor: next}
	updatedDigest, err := CheckpointFloorDigestV068(updated)
	if err != nil {
		return reject(err)
	}
	return updated, TrustedCheckpointFloorWitnessV068{
		Revision: updated.Revision,
		Digest:   updatedDigest,
	}, nil
}
