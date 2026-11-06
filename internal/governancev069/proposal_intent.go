// Package governancev069 defines advisory, non-executable governance proposal
// intents. This package intentionally has no consensus or signing integration.
package governancev069

import (
    "crypto/sha256"
    "encoding/binary"
    "encoding/hex"
    "errors"
    "fmt"
    "hash"
    "strings"
    "unicode"
    "unicode/utf8"
)

const (
    ProtocolVersion = "PRISM_GOV_V069_PROPOSAL_INTENT_V1"
    commitmentDomain = "PRISM-GOV-v0.69/PROPOSAL-INTENT/v1\x00"
    replayDomain = "PRISM-GOV-v0.69/PROPOSER-NONCE/v1\x00"
)

type Topic string

const (
    TopicDiscussion       Topic = "DISCUSSION"
    TopicParameterReview  Topic = "PARAMETER_REVIEW"
    TopicUpgradeReview    Topic = "UPGRADE_REVIEW"
    TopicTreasuryReview   Topic = "TREASURY_REVIEW"
)

// ProposalIntent is a discussion artifact, NOT a consensus transaction,
// authorization, on-chain proposal, validator vote, or executable instruction.
// Text is committed as raw UTF-8 bytes without Unicode normalization.
type ProposalIntent struct {
    Version         string `json:"version"`
    Network         string `json:"network"`
    ChainID         string `json:"chain_id"`
    GenesisSHA256   string `json:"genesis_sha256"`
    Proposer        string `json:"proposer"`
    Nonce           uint64 `json:"nonce"`
    CreatedAtHeight uint64 `json:"created_at_height"`
    ExpiresAtHeight uint64 `json:"expires_at_height"`
    Topic           Topic  `json:"topic"`
    Title           string `json:"title"`
    Rationale       string `json:"rationale"`
    PayloadSHA256   string `json:"payload_sha256"`
}

// Context is supplied by a trusted caller; it is NOT inferred from the intent.
type Context struct {
    Network       string
    ChainID       string
    GenesisSHA256 string
    CurrentHeight uint64
}

func ValidateShape(p ProposalIntent) error {
    if p.Version != ProtocolVersion { return errors.New("invalid proposal intent version") }
    if !asciiIdentifier(p.Network, 1, 48) { return errors.New("invalid network") }
    if !asciiIdentifier(p.ChainID, 1, 128) { return errors.New("invalid chain ID") }
    if !isLowerSHA256(p.GenesisSHA256) { return errors.New("invalid genesis SHA-256") }
    if !asciiIdentifier(p.Proposer, 3, 128) { return errors.New("invalid proposer") }
    if p.Nonce == 0 { return errors.New("nonce must be nonzero") }
    if p.CreatedAtHeight == 0 || p.ExpiresAtHeight <= p.CreatedAtHeight {
        return errors.New("invalid height window")
    }
    switch p.Topic {
    case TopicDiscussion, TopicParameterReview, TopicUpgradeReview, TopicTreasuryReview:
    default: return errors.New("unsupported advisory topic")
    }
    if !cleanText(p.Title, 8, 120) { return errors.New("invalid title") }
    if !cleanText(p.Rationale, 16, 2048) { return errors.New("invalid rationale") }
    if !isLowerSHA256(p.PayloadSHA256) { return errors.New("invalid payload SHA-256") }
    return nil
}

// ValidateForContext fails closed on wrong network/genesis and expired/future
// intents. Validity does NOT authorize admission, voting or execution.
func ValidateForContext(p ProposalIntent, c Context) error {
    if err := ValidateShape(p); err != nil { return err }
    if p.Network != c.Network || p.ChainID != c.ChainID || p.GenesisSHA256 != c.GenesisSHA256 {
        return errors.New("network, chain or genesis mismatch")
    }
    if c.CurrentHeight < p.CreatedAtHeight { return errors.New("intent not yet valid") }
    if c.CurrentHeight > p.ExpiresAtHeight { return errors.New("intent expired") }
    return nil
}

// Commitment returns a domain-separated SHA-256 digest over a stable,
// explicit length-prefixed binary encoding. It has no signature semantics.
func Commitment(p ProposalIntent) ([32]byte, error) {
    if err := ValidateShape(p); err != nil { return [32]byte{}, err }
    h := sha256.New()
    h.Write([]byte(commitmentDomain))
    writeString(h, p.Version)
    writeString(h, p.Network)
    writeString(h, p.ChainID)
    writeString(h, p.GenesisSHA256)
    writeString(h, p.Proposer)
    writeUint64(h, p.Nonce)
    writeUint64(h, p.CreatedAtHeight)
    writeUint64(h, p.ExpiresAtHeight)
    writeString(h, string(p.Topic))
    writeString(h, p.Title)
    writeString(h, p.Rationale)
    writeString(h, p.PayloadSHA256)
    var out [32]byte
    copy(out[:], h.Sum(nil))
    return out, nil
}

// ReplayKey identifies a (network, chain, genesis, proposer, nonce) tuple.
// A persistent atomic store is REQUIRED to enforce uniqueness; calculating a
// replay key alone does NOT prevent replay or concurrent admissions.
func ReplayKey(p ProposalIntent) ([32]byte, error) {
    if err := ValidateShape(p); err != nil { return [32]byte{}, err }
    h := sha256.New()
    h.Write([]byte(replayDomain))
    writeString(h, p.Network)
    writeString(h, p.ChainID)
    writeString(h, p.GenesisSHA256)
    writeString(h, p.Proposer)
    writeUint64(h, p.Nonce)
    var out [32]byte
    copy(out[:], h.Sum(nil))
    return out, nil
}

func writeString(h hash.Hash, v string) {
    var n [4]byte
    binary.BigEndian.PutUint32(n[:], uint32(len(v)))
    h.Write(n[:])
    h.Write([]byte(v))
}

func writeUint64(h hash.Hash, v uint64) {
    var n [8]byte
    binary.BigEndian.PutUint64(n[:], v)
    h.Write(n[:])
}

func isLowerSHA256(s string) bool {
    if len(s) != 64 { return false }
    _, err := hex.DecodeString(s)
    if err != nil { return false }
    for _, c := range s {
        if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) { return false }
    }
    return true
}

func asciiIdentifier(s string, min, max int) bool {
    if len(s) < min || len(s) > max { return false }
    for _, c := range s {
        if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
            (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ':') {
            return false
        }
    }
    return true
}

func cleanText(s string, minBytes, maxBytes int) bool {
    if !utf8.ValidString(s) || len(s) < minBytes || len(s) > maxBytes || strings.TrimSpace(s) != s { return false }
    for _, r := range s { if unicode.IsControl(r) { return false } }
    return true
}

// HexCommitment is a convenience accessor, with no execution side effects.
func HexCommitment(p ProposalIntent) (string, error) {
    digest, err := Commitment(p)
    if err != nil { return "", fmt.Errorf("proposal commitment: %w", err) }
    return hex.EncodeToString(digest[:]), nil
}
