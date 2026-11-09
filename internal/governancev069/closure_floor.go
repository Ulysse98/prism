package governancev069

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

const (
	AdvisoryClosureRecoveryFloorVersion = "PRISM_GOV_V069_CLOSURE_RECOVERY_FLOOR_V1"
	closureRecoveryFloorDomain          = "PRISM-GOV-v0.69/CLOSURE-RECOVERY-FLOOR/v1\x00"
)

// AdvisoryClosureRecoveryFloor is a pinned, exact signed-marker checkpoint.
// It MUST be retained and supplied from an independently trusted storage or
// operator channel. Its self-checking SHA256 is not a signature and cannot
// protect an anchor stored and rolled back with the voting directory.
// One floor is scoped to ONE proposal, not the whole chain.
type AdvisoryClosureRecoveryFloor struct {
	Version               string `json:"version"`
	Network               string `json:"network"`
	ChainID               string `json:"chain_id"`
	GenesisSHA256         string `json:"genesis_sha256"`
	ProposalSHA256        string `json:"proposal_sha256"`
	ClosureSHA256         string `json:"closure_sha256"`
	ClosedAtHeight        uint64 `json:"closed_at_height"`
	AuthorityPublicKeyHex string `json:"authority_public_key_hex"`
	AuthorityEpoch        uint64 `json:"authority_epoch"`
	MarkerSHA256          string `json:"marker_sha256"`
	FloorSHA256           string `json:"floor_sha256"`
}

func closureMarkerDigest(c AdvisoryBallotClosure) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func closureFloorCommitment(f AdvisoryClosureRecoveryFloor) [32]byte {
	h := sha256.New()
	h.Write([]byte(closureRecoveryFloorDomain))
	writeString(h, f.Version)
	writeString(h, f.Network)
	writeString(h, f.ChainID)
	writeString(h, f.GenesisSHA256)
	writeString(h, f.ProposalSHA256)
	writeString(h, f.ClosureSHA256)
	writeUint64(h, f.ClosedAtHeight)
	writeString(h, f.AuthorityPublicKeyHex)
	writeUint64(h, f.AuthorityEpoch)
	writeString(h, f.MarkerSHA256)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// BuildAdvisoryClosureRecoveryFloor must be called only on a closure already
// checked against a separate, pinned operator key. The epoch is a trusted
// operator-side value; it is not established by the local ballot store.
func BuildAdvisoryClosureRecoveryFloor(c AdvisoryBallotClosure, pinnedAuthorityPublicKeyHex string, authorityEpoch uint64) (AdvisoryClosureRecoveryFloor, error) {
	if authorityEpoch == 0 {
		return AdvisoryClosureRecoveryFloor{}, errors.New("authority epoch must be positive")
	}
	if err := VerifyAdvisoryClosureAuthorization(c, c.Authorization, pinnedAuthorityPublicKeyHex); err != nil {
		return AdvisoryClosureRecoveryFloor{}, err
	}
	marker, err := closureMarkerDigest(c)
	if err != nil {
		return AdvisoryClosureRecoveryFloor{}, err
	}
	f := AdvisoryClosureRecoveryFloor{
		Version: AdvisoryClosureRecoveryFloorVersion,
		Network: c.Network, ChainID: c.ChainID,
		GenesisSHA256: c.GenesisSHA256, ProposalSHA256: c.ProposalSHA256,
		ClosureSHA256: c.ClosureSHA256, ClosedAtHeight: c.ClosedAtHeight,
		AuthorityPublicKeyHex: pinnedAuthorityPublicKeyHex,
		AuthorityEpoch:        authorityEpoch, MarkerSHA256: marker,
	}
	d := closureFloorCommitment(f)
	f.FloorSHA256 = hex.EncodeToString(d[:])
	return f, nil
}

// ValidateAdvisoryClosureRecoveryFloor checks canonical shape and checksum.
// It does NOT establish that the caller's floor is an independently trusted
// checkpoint: the caller must provide and preserve that independent pin.
func ValidateAdvisoryClosureRecoveryFloor(f AdvisoryClosureRecoveryFloor) error {
	if f.Version != AdvisoryClosureRecoveryFloorVersion ||
		!asciiIdentifier(f.Network, 1, 48) || !asciiIdentifier(f.ChainID, 1, 128) ||
		!isLowerSHA256(f.GenesisSHA256) || !isLowerSHA256(f.ProposalSHA256) ||
		!isLowerSHA256(f.ClosureSHA256) || !isLowerSHA256(f.AuthorityPublicKeyHex) ||
		!isLowerSHA256(f.MarkerSHA256) || !isLowerSHA256(f.FloorSHA256) ||
		f.ClosedAtHeight == 0 || f.AuthorityEpoch == 0 {
		return errors.New("invalid recovery floor shape")
	}
	d := closureFloorCommitment(f)
	if f.FloorSHA256 != hex.EncodeToString(d[:]) {
		return errors.New("recovery floor commitment mismatch")
	}
	return nil
}

// VerifyAdvisoryClosureAgainstFloor rejects absent/altered closures or
// a different signing authority, even when the replacement closure has a
// valid signature. A trusted external floor is an essential precondition.
func VerifyAdvisoryClosureAgainstFloor(c AdvisoryBallotClosure, f AdvisoryClosureRecoveryFloor) error {
	if err := ValidateAdvisoryClosureRecoveryFloor(f); err != nil {
		return err
	}
	if err := VerifyAdvisoryClosureAuthorization(c, c.Authorization, f.AuthorityPublicKeyHex); err != nil {
		return err
	}
	marker, err := closureMarkerDigest(c)
	if err != nil {
		return err
	}
	if c.Network != f.Network || c.ChainID != f.ChainID ||
		c.GenesisSHA256 != f.GenesisSHA256 || c.ProposalSHA256 != f.ProposalSHA256 ||
		c.ClosureSHA256 != f.ClosureSHA256 || c.ClosedAtHeight != f.ClosedAtHeight ||
		marker != f.MarkerSHA256 {
		return errors.New("signed local closure differs from independently pinned recovery floor")
	}
	return nil
}
