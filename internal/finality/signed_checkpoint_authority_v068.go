package finality

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

const checkpointDomainV068 = "prism-finality-v068/signed-checkpoint/v1\x00"

// CheckpointAuthorityV068 is a caller-trusted operator key with height-based
// validity bounds. No historical validator voting authority is derived here.
// The key and revocation status MUST be obtained from an external, independently
// authenticated policy; never accept them from the same untrusted peer as the
// checkpoint under verification.
type CheckpointAuthorityV068 struct {
	ID                string
	PublicKey         ed25519.PublicKey
	FirstParentHeight uint64
	LastParentHeight  uint64
	Revoked           bool
}

// CheckpointTrustPolicyV068 is an externally established, local-only trust root.
// MinParentHeight/MaxParentHeight are trusted height bounds, NOT wall-clock
// expiry, automatically persisted anti-replay state, or chain finality.
type CheckpointTrustPolicyV068 struct {
	ChainID         string
	GenesisHash     string
	MinParentHeight uint64
	MaxParentHeight uint64
	Authorities     []CheckpointAuthorityV068
}

// SignedValidatorCheckpointV068 carries a previously signed external pin.
// This package intentionally does not expose a checkpoint-signing API.
type SignedValidatorCheckpointV068 struct {
	Anchor       PinnedValidatorAnchorV068
	AuthorityID  string
	SignatureHex string
}

func validAuthorityIDV068(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		b := id[i]
		if !((b >= 'a' && b <= 'z') ||
			(b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.') {
			return false
		}
	}
	return true
}

// signedCheckpointPayloadV068 is a versioned, domain-separated wire contract.
// Only tests use the helper to construct throwaway signatures. Vote signing,
// validator signing and production checkpoint issuance are out of scope.
func signedCheckpointPayloadV068(s SignedValidatorCheckpointV068) ([]byte, error) {
	a := s.Anchor
	if !validAuthorityIDV068(s.AuthorityID) || a.ChainID == "" ||
		!validHash(a.GenesisHash) || !validHash(a.ParentHash) ||
		!validHash(a.ValidatorSetHash) {
		return nil, errors.New("invalid canonical signed checkpoint context")
	}
	wire := struct {
		AuthorityID      string `json:"authority_id"`
		ChainID          string `json:"chain_id"`
		GenesisHash      string `json:"genesis_hash"`
		ParentHeight     uint64 `json:"parent_height"`
		ParentHash       string `json:"parent_hash"`
		ValidatorSetHash string `json:"validator_set_hash"`
	}{
		AuthorityID: s.AuthorityID, ChainID: a.ChainID,
		GenesisHash: a.GenesisHash, ParentHeight: a.ParentHeight,
		ParentHash: a.ParentHash, ValidatorSetHash: a.ValidatorSetHash,
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode signed checkpoint: %w", err)
	}
	return append([]byte(checkpointDomainV068), data...), nil
}

// VerifySignedPinnedValidatorSnapshotV068 verifies an operator signature
// under a caller-authenticated policy, then reuses the existing independent
// parent-chain and current locked-stake checks from Phase 1B.1.
// It never signs, selects a trusted policy from peer input, stores replay
// history, creates a vote, changes active consensus, or establishes finality.
func VerifySignedPinnedValidatorSnapshotV068(
	parent *blockchain.Blockchain,
	validators []consensus.Validator,
	signed SignedValidatorCheckpointV068,
	policy CheckpointTrustPolicyV068,
) error {
	if policy.ChainID == "" || !validHash(policy.GenesisHash) ||
		policy.MinParentHeight > policy.MaxParentHeight ||
		len(policy.Authorities) == 0 || len(policy.Authorities) > 64 {
		return errors.New("invalid independently trusted checkpoint policy")
	}
	if signed.Anchor.ChainID != policy.ChainID ||
		signed.Anchor.GenesisHash != policy.GenesisHash ||
		signed.Anchor.ParentHeight < policy.MinParentHeight ||
		signed.Anchor.ParentHeight > policy.MaxParentHeight {
		return errors.New("checkpoint disagrees with trusted network or height bounds")
	}

	seenIDs := make(map[string]bool, len(policy.Authorities))
	seenKeys := make(map[string]bool, len(policy.Authorities))
	var signer *CheckpointAuthorityV068
	for i := range policy.Authorities {
		a := &policy.Authorities[i]
		if !validAuthorityIDV068(a.ID) || len(a.PublicKey) != ed25519.PublicKeySize ||
			a.FirstParentHeight > a.LastParentHeight {
			return errors.New("invalid trusted authority entry")
		}
		keyHex := hex.EncodeToString(a.PublicKey)
		if seenIDs[a.ID] || seenKeys[keyHex] {
			return errors.New("ambiguous trusted authority registry")
		}
		seenIDs[a.ID] = true
		seenKeys[keyHex] = true
		if a.ID == signed.AuthorityID {
			signer = a
		}
	}
	if signer == nil || signer.Revoked ||
		signed.Anchor.ParentHeight < signer.FirstParentHeight ||
		signed.Anchor.ParentHeight > signer.LastParentHeight {
		return errors.New("checkpoint authority unknown, revoked, or outside trusted height window")
	}

	payload, err := signedCheckpointPayloadV068(signed)
	if err != nil {
		return err
	}
	sig, err := hex.DecodeString(signed.SignatureHex)
	if err != nil || len(sig) != ed25519.SignatureSize ||
		hex.EncodeToString(sig) != signed.SignatureHex {
		return errors.New("invalid canonical Ed25519 checkpoint signature")
	}
	if !ed25519.Verify(signer.PublicKey, payload, sig) {
		return errors.New("checkpoint signature failed verification")
	}
	if err := VerifyPinnedValidatorSnapshotV068(parent, validators, signed.Anchor); err != nil {
		return fmt.Errorf("signed checkpoint parent verification: %w", err)
	}
	return nil
}
