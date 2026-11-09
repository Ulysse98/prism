package governancev069

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	AdvisoryClosureAuthorityVersion = "PRISM_GOV_V069_CLOSURE_AUTHORITY_V1"
	closureAuthorityDomain          = "PRISM-GOV-v0.69/CLOSURE-AUTHORITY/v1\x00"
)

// AdvisoryClosureAuthorization binds one operator's Ed25519 signature to one
// exact independently computed local closure digest. The public key inside
// the record is not a trust anchor; callers MUST supply a separate pinned key.
// There is no on-chain or distributed authorization implied by this struct.
type AdvisoryClosureAuthorization struct {
	Version            string `json:"version"`
	SignerPublicKeyHex string `json:"signer_public_key_hex"`
	ClosureSHA256      string `json:"closure_sha256"`
	Nonce              uint64 `json:"nonce"`
	SignatureHex       string `json:"signature_hex"`
}

func closureAuthorityDigest(a AdvisoryClosureAuthorization) [32]byte {
	h := sha256.New()
	h.Write([]byte(closureAuthorityDomain))
	writeString(h, a.Version)
	writeString(h, a.SignerPublicKeyHex)
	writeString(h, a.ClosureSHA256)
	writeUint64(h, a.Nonce)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func validateClosureBody(c AdvisoryBallotClosure) error {
	if c.Version != AdvisoryBallotClosureVersion ||
		!asciiIdentifier(c.Network, 1, 48) || !asciiIdentifier(c.ChainID, 1, 128) ||
		!isLowerSHA256(c.GenesisSHA256) || !isLowerSHA256(c.ProposalSHA256) ||
		!isLowerSHA256(c.SnapshotSHA256) || !isLowerSHA256(c.BallotSetSHA256) ||
		!isLowerSHA256(c.TallyReceiptSHA256) || !isLowerSHA256(c.ReservationAuditSHA256) ||
		!isLowerSHA256(c.ClosureSHA256) || c.ClosedAtHeight == 0 {
		return errors.New("invalid canonical closure body")
	}
	digest := localBallotClosureCommitment(c)
	if c.ClosureSHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("closure SHA-256 mismatch")
	}
	return nil
}

// SignAdvisoryClosureAuthorization signs a precomputed, audited closure draft.
// Callers must keep their operator key offline and pin the corresponding public
// key independently. This helper is NOT a key-management or policy authority.
func SignAdvisoryClosureAuthorization(c AdvisoryBallotClosure, nonce uint64, privateKey ed25519.PrivateKey) (AdvisoryClosureAuthorization, error) {
	if err := validateClosureBody(c); err != nil {
		return AdvisoryClosureAuthorization{}, err
	}
	if nonce == 0 || len(privateKey) != ed25519.PrivateKeySize {
		return AdvisoryClosureAuthorization{}, errors.New("nonzero nonce and valid Ed25519 key required")
	}
	canonicalKey := ed25519.NewKeyFromSeed(privateKey.Seed())
	publicKey := canonicalKey.Public().(ed25519.PublicKey)
	a := AdvisoryClosureAuthorization{
		Version:            AdvisoryClosureAuthorityVersion,
		SignerPublicKeyHex: hex.EncodeToString(publicKey),
		ClosureSHA256:      c.ClosureSHA256,
		Nonce:              nonce,
	}
	digest := closureAuthorityDigest(a)
	a.SignatureHex = hex.EncodeToString(ed25519.Sign(canonicalKey, digest[:]))
	return a, nil
}

// VerifyAdvisoryClosureAuthorization requires a pinned key from an independent
// trust decision, not a key copied from the unsigned closure or this record.
func VerifyAdvisoryClosureAuthorization(c AdvisoryBallotClosure, a AdvisoryClosureAuthorization, pinnedPublicKeyHex string) error {
	if err := validateClosureBody(c); err != nil {
		return err
	}
	if a.Version != AdvisoryClosureAuthorityVersion || a.Nonce == 0 ||
		!isLowerSHA256(a.SignerPublicKeyHex) || !isLowerSHA256(pinnedPublicKeyHex) ||
		a.SignerPublicKeyHex != pinnedPublicKeyHex || a.ClosureSHA256 != c.ClosureSHA256 ||
		len(a.SignatureHex) != ed25519.SignatureSize*2 || strings.ToLower(a.SignatureHex) != a.SignatureHex {
		return errors.New("closure authority version, identity, nonce, signature shape or pin mismatch")
	}
	sig, err := hex.DecodeString(a.SignatureHex)
	if err != nil {
		return fmt.Errorf("closure authority signature: %w", err)
	}
	pub, err := hex.DecodeString(a.SignerPublicKeyHex)
	if err != nil {
		return fmt.Errorf("closure authority public key: %w", err)
	}
	digest := closureAuthorityDigest(a)
	if !ed25519.Verify(ed25519.PublicKey(pub), digest[:], sig) {
		return errors.New("invalid closure authority signature")
	}
	return nil
}

// closureBodyEqual excludes the separately validated authority proof.
func closureBodyEqual(a, b AdvisoryBallotClosure) bool {
	a.Authorization = AdvisoryClosureAuthorization{}
	b.Authorization = AdvisoryClosureAuthorization{}
	return a == b
}
