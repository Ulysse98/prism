package governancev069

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"strings"
)

const (
	AdvisoryAuthorityRotationVersion = "PRISM_GOV_V069_AUTHORITY_ROTATION_CANDIDATE_V1"
	authorityRotationDomain          = "PRISM-GOV-v0.69/AUTHORITY-ROTATION-CANDIDATE/v1\x00"
)

// AdvisoryAuthorityRotation is an OFFLINE dual-signature transition proposal.
// It is NOT activated by verification. No vote store, authorization verifier,
// or consensus key changes when this record is constructed or checked.
type AdvisoryAuthorityRotation struct {
	Version             string `json:"version"`
	Network             string `json:"network"`
	ChainID             string `json:"chain_id"`
	GenesisSHA256       string `json:"genesis_sha256"`
	ProposalSHA256      string `json:"proposal_sha256"`
	ClosureSHA256       string `json:"closure_sha256"`
	RecoveryFloorSHA256 string `json:"recovery_floor_sha256"`
	FromAuthorityHex    string `json:"from_authority_hex"`
	ToAuthorityHex      string `json:"to_authority_hex"`
	FromEpoch           uint64 `json:"from_epoch"`
	ToEpoch             uint64 `json:"to_epoch"`
	Nonce               uint64 `json:"nonce"`
	OldSignatureHex     string `json:"old_signature_hex"`
	NewSignatureHex     string `json:"new_signature_hex"`
}

func authorityRotationDigest(r AdvisoryAuthorityRotation) [32]byte {
	h := sha256.New()
	h.Write([]byte(authorityRotationDomain))
	writeString(h, r.Version)
	writeString(h, r.Network)
	writeString(h, r.ChainID)
	writeString(h, r.GenesisSHA256)
	writeString(h, r.ProposalSHA256)
	writeString(h, r.ClosureSHA256)
	writeString(h, r.RecoveryFloorSHA256)
	writeString(h, r.FromAuthorityHex)
	writeString(h, r.ToAuthorityHex)
	writeUint64(h, r.FromEpoch)
	writeUint64(h, r.ToEpoch)
	writeUint64(h, r.Nonce)
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

func validRotationShape(r AdvisoryAuthorityRotation) error {
	if r.Version != AdvisoryAuthorityRotationVersion ||
		!asciiIdentifier(r.Network, 1, 48) || !asciiIdentifier(r.ChainID, 1, 128) ||
		!isLowerSHA256(r.GenesisSHA256) || !isLowerSHA256(r.ProposalSHA256) ||
		!isLowerSHA256(r.ClosureSHA256) || !isLowerSHA256(r.RecoveryFloorSHA256) || !isLowerSHA256(r.FromAuthorityHex) ||
		!isLowerSHA256(r.ToAuthorityHex) || r.FromAuthorityHex == r.ToAuthorityHex ||
		r.FromEpoch == 0 || r.FromEpoch == math.MaxUint64 || r.ToEpoch != r.FromEpoch+1 || r.Nonce == 0 ||
		len(r.OldSignatureHex) != ed25519.SignatureSize*2 || len(r.NewSignatureHex) != ed25519.SignatureSize*2 ||
		strings.ToLower(r.OldSignatureHex) != r.OldSignatureHex || strings.ToLower(r.NewSignatureHex) != r.NewSignatureHex {
		return errors.New("invalid authority rotation candidate shape")
	}
	return nil
}

// SignAdvisoryAuthorityRotationCandidate requires the already externally
// pinned floor, an old-key signature, and a new-key proof of possession.
// Private keys are never written to the rotation record.
func SignAdvisoryAuthorityRotationCandidate(f AdvisoryClosureRecoveryFloor, newAuthorityKey ed25519.PrivateKey, oldAuthorityKey ed25519.PrivateKey, nonce uint64) (AdvisoryAuthorityRotation, error) {
	if err := ValidateAdvisoryClosureRecoveryFloor(f); err != nil {
		return AdvisoryAuthorityRotation{}, err
	}
	if len(oldAuthorityKey) != ed25519.PrivateKeySize || len(newAuthorityKey) != ed25519.PrivateKeySize || nonce == 0 || f.AuthorityEpoch == math.MaxUint64 {
		return AdvisoryAuthorityRotation{}, errors.New("invalid rotation keys, epoch or nonce")
	}
	old := ed25519.NewKeyFromSeed(oldAuthorityKey.Seed())
	next := ed25519.NewKeyFromSeed(newAuthorityKey.Seed())
	oldPub := hex.EncodeToString(old.Public().(ed25519.PublicKey))
	newPub := hex.EncodeToString(next.Public().(ed25519.PublicKey))
	if oldPub != f.AuthorityPublicKeyHex || newPub == oldPub {
		return AdvisoryAuthorityRotation{}, errors.New("rotation not rooted in pinned authority")
	}
	r := AdvisoryAuthorityRotation{
		Version: AdvisoryAuthorityRotationVersion,
		Network: f.Network, ChainID: f.ChainID, GenesisSHA256: f.GenesisSHA256,
		ProposalSHA256: f.ProposalSHA256, ClosureSHA256: f.ClosureSHA256, RecoveryFloorSHA256: f.FloorSHA256,
		FromAuthorityHex: oldPub, ToAuthorityHex: newPub,
		FromEpoch: f.AuthorityEpoch, ToEpoch: f.AuthorityEpoch + 1, Nonce: nonce,
	}
	digest := authorityRotationDigest(r)
	r.OldSignatureHex = hex.EncodeToString(ed25519.Sign(old, digest[:]))
	r.NewSignatureHex = hex.EncodeToString(ed25519.Sign(next, digest[:]))
	return r, nil
}

// VerifyAdvisoryAuthorityRotationCandidate checks the proposed next key
// against an independently pinned prior floor. It does NOT rotate any keys.
func VerifyAdvisoryAuthorityRotationCandidate(r AdvisoryAuthorityRotation, trustedFloor AdvisoryClosureRecoveryFloor) error {
	if err := ValidateAdvisoryClosureRecoveryFloor(trustedFloor); err != nil {
		return err
	}
	if err := validRotationShape(r); err != nil {
		return err
	}
	if r.Network != trustedFloor.Network || r.ChainID != trustedFloor.ChainID ||
		r.GenesisSHA256 != trustedFloor.GenesisSHA256 || r.ProposalSHA256 != trustedFloor.ProposalSHA256 ||
		r.ClosureSHA256 != trustedFloor.ClosureSHA256 || r.RecoveryFloorSHA256 != trustedFloor.FloorSHA256 || r.FromAuthorityHex != trustedFloor.AuthorityPublicKeyHex ||
		r.FromEpoch != trustedFloor.AuthorityEpoch {
		return errors.New("rotation candidate does not match trusted prior floor")
	}
	from, err := hex.DecodeString(r.FromAuthorityHex)
	if err != nil {
		return err
	}
	to, err := hex.DecodeString(r.ToAuthorityHex)
	if err != nil {
		return err
	}
	oldSig, err := hex.DecodeString(r.OldSignatureHex)
	if err != nil {
		return err
	}
	newSig, err := hex.DecodeString(r.NewSignatureHex)
	if err != nil {
		return err
	}
	digest := authorityRotationDigest(r)
	if !ed25519.Verify(from, digest[:], oldSig) || !ed25519.Verify(to, digest[:], newSig) {
		return errors.New("rotation candidate lacks valid dual signatures")
	}
	return nil
}
