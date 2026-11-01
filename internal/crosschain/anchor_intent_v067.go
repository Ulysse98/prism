package crosschain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"regexp"
)

const AnchorIntentVersionV067 uint32 = 1

const anchorIntentDomainV067 = "PRISM/CROSSCHAIN/ANCHOR-INTENT/V067\x00"

var (
	canonicalHashV067 = regexp.MustCompile(`^[0-9a-f]{64}$`)
	chainIDV067       = regexp.MustCompile(`^[a-z][a-z0-9._:-]{0,127}$`)
	registryIDV067    = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
)

// AnchorIntentV067 binds a candidate Prism compute proof to one destination.
// It is NOT evidence of proof verification, source finality, authority, or payment.
// On-chain settlement must never rely on this structure or digest alone.
type AnchorIntentV067 struct {
	Version             uint32 `json:"version"`
	SourceChainID       string `json:"source_chain_id"`
	SourceGenesisHash   string `json:"source_genesis_hash"`
	SourceBlockHeight   uint64 `json:"source_block_height"`
	SourceBlockHash     string `json:"source_block_hash"`
	JobID               string `json:"job_id"`
	ProofHash           string `json:"proof_hash"`
	DestinationChainID  string `json:"destination_chain_id"`
	DestinationRegistry string `json:"destination_registry"`
	Nonce               uint64 `json:"nonce"`
}

func (a AnchorIntentV067) Validate() error {
	if a.Version != AnchorIntentVersionV067 {
		return errors.New("anchor intent: unsupported version")
	}
	if !chainIDV067.MatchString(a.SourceChainID) {
		return errors.New("anchor intent: invalid source chain ID")
	}
	if err := validHashV067("source genesis hash", a.SourceGenesisHash); err != nil {
		return err
	}
	if a.SourceBlockHeight == 0 {
		return errors.New("anchor intent: zero source block height")
	}
	if err := validHashV067("source block hash", a.SourceBlockHash); err != nil {
		return err
	}
	if err := validHashV067("job ID", a.JobID); err != nil {
		return err
	}
	if err := validHashV067("proof hash", a.ProofHash); err != nil {
		return err
	}
	if !chainIDV067.MatchString(a.DestinationChainID) {
		return errors.New("anchor intent: invalid destination chain ID")
	}
	if !registryIDV067.MatchString(a.DestinationRegistry) {
		return errors.New("anchor intent: invalid destination registry ID")
	}
	if a.Nonce == 0 {
		return errors.New("anchor intent: zero nonce")
	}
	return nil
}

func validHashV067(name, value string) error {
	if !canonicalHashV067.MatchString(value) || value == "0000000000000000000000000000000000000000000000000000000000000000" {
		return fmt.Errorf("anchor intent: invalid canonical %s", name)
	}
	return nil
}

// Digest returns an unambiguous, domain-separated SHA-256 commitment to the intent.
// All string fields are length-prefixed, and integers use fixed-width big-endian.
func (a AnchorIntentV067) Digest() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(anchorIntentDomainV067))
	writeU32V067(h, a.Version)
	writeFieldV067(h, a.SourceChainID)
	writeFieldV067(h, a.SourceGenesisHash)
	writeU64V067(h, a.SourceBlockHeight)
	writeFieldV067(h, a.SourceBlockHash)
	writeFieldV067(h, a.JobID)
	writeFieldV067(h, a.ProofHash)
	writeFieldV067(h, a.DestinationChainID)
	writeFieldV067(h, a.DestinationRegistry)
	writeU64V067(h, a.Nonce)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeFieldV067(h hash.Hash, s string) {
	writeU32V067(h, uint32(len(s)))
	_, _ = h.Write([]byte(s))
}

func writeU32V067(h hash.Hash, n uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], n)
	_, _ = h.Write(buf[:])
}

func writeU64V067(h hash.Hash, n uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	_, _ = h.Write(buf[:])
}
