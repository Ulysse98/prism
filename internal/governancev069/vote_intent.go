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
	AdvisoryVoteProtocolVersion = "PRISM_GOV_V069_ADVISORY_VOTE_V1"
	voteCommitmentDomain        = "PRISM-GOV-v0.69/ADVISORY-VOTE/v1\x00"
	voteReplayDomain            = "PRISM-GOV-v0.69/VOTER-PROPOSAL/v1\x00"
)

// VoteChoice is an advisory preference. No consensus/voting right is implied.
type VoteChoice string

const (
	VoteYes     VoteChoice = "YES"
	VoteNo      VoteChoice = "NO"
	VoteAbstain VoteChoice = "ABSTAIN"
)

// AdvisoryVoteIntent is a fully context-bound, advisory vote statement.
// VoterPublicKeyHex is the claimed public key, NOT proof of eligibility.
// Nonce does NOT allow multiple votes: ReplayKey is voter+proposal scoped.
type AdvisoryVoteIntent struct {
	Version           string     `json:"version"`
	Network           string     `json:"network"`
	ChainID           string     `json:"chain_id"`
	GenesisSHA256     string     `json:"genesis_sha256"`
	ProposalSHA256    string     `json:"proposal_sha256"`
	VoterPublicKeyHex string     `json:"voter_public_key_hex"`
	Nonce             uint64     `json:"nonce"`
	CreatedAtHeight   uint64     `json:"created_at_height"`
	ExpiresAtHeight   uint64     `json:"expires_at_height"`
	Choice            VoteChoice `json:"choice"`
}

// SignedAdvisoryVote is an offline Ed25519 attestation. It is not a validator
// signature, a token snapshot, a quorum proof or an execution authorization.
type SignedAdvisoryVote struct {
	Vote         AdvisoryVoteIntent `json:"vote"`
	SignatureHex string             `json:"signature_hex"`
}

func ValidateVoteShape(v AdvisoryVoteIntent) error {
	if v.Version != AdvisoryVoteProtocolVersion {
		return errors.New("unsupported advisory vote version")
	}
	if !asciiIdentifier(v.Network, 1, 48) || !asciiIdentifier(v.ChainID, 1, 128) {
		return errors.New("invalid vote network or chain")
	}
	if !isLowerSHA256(v.GenesisSHA256) || !isLowerSHA256(v.ProposalSHA256) {
		return errors.New("invalid vote genesis or proposal digest")
	}
	if !isLowerSHA256(v.VoterPublicKeyHex) {
		return errors.New("invalid lowercase Ed25519 public key")
	}
	if v.Nonce == 0 {
		return errors.New("vote nonce must be nonzero")
	}
	if v.CreatedAtHeight == 0 || v.ExpiresAtHeight <= v.CreatedAtHeight {
		return errors.New("invalid vote height window")
	}
	switch v.Choice {
	case VoteYes, VoteNo, VoteAbstain:
	default:
		return errors.New("invalid advisory choice")
	}
	return nil
}

// ValidateVoteForProposal uses a trusted context and an actual proposal value,
// rather than trusting a proposal ID supplied by the vote itself.
func ValidateVoteForProposal(v AdvisoryVoteIntent, proposal ProposalIntent, trusted Context) error {
	if err := ValidateVoteShape(v); err != nil {
		return err
	}
	if err := ValidateForContext(proposal, trusted); err != nil {
		return fmt.Errorf("proposal context: %w", err)
	}
	if v.Network != trusted.Network || v.ChainID != trusted.ChainID || v.GenesisSHA256 != trusted.GenesisSHA256 {
		return errors.New("advisory vote network/chain/genesis mismatch")
	}
	digest, err := Commitment(proposal)
	if err != nil {
		return fmt.Errorf("proposal commitment: %w", err)
	}
	if v.ProposalSHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("advisory vote bound to wrong proposal")
	}
	if v.CreatedAtHeight < proposal.CreatedAtHeight || v.ExpiresAtHeight > proposal.ExpiresAtHeight {
		return errors.New("vote window exceeds proposal window")
	}
	if trusted.CurrentHeight < v.CreatedAtHeight || trusted.CurrentHeight > v.ExpiresAtHeight {
		return errors.New("advisory vote outside active height window")
	}
	return nil
}

func voteDigest(v AdvisoryVoteIntent) ([32]byte, error) {
	if err := ValidateVoteShape(v); err != nil {
		return [32]byte{}, err
	}
	h := sha256.New()
	h.Write([]byte(voteCommitmentDomain))
	writeString(h, v.Version)
	writeString(h, v.Network)
	writeString(h, v.ChainID)
	writeString(h, v.GenesisSHA256)
	writeString(h, v.ProposalSHA256)
	writeString(h, v.VoterPublicKeyHex)
	writeUint64(h, v.Nonce)
	writeUint64(h, v.CreatedAtHeight)
	writeUint64(h, v.ExpiresAtHeight)
	writeString(h, string(v.Choice))
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

// AdvisoryVoteCommitment is a reproducible SHA-256 digest (not a signature).
func AdvisoryVoteCommitment(v AdvisoryVoteIntent) ([32]byte, error) { return voteDigest(v) }

// SignAdvisoryVote signs the 32-byte domain-separated digest using standard
// Ed25519. Only offline user-supplied keys are accepted; no key storage.
func SignAdvisoryVote(v AdvisoryVoteIntent, key ed25519.PrivateKey) (SignedAdvisoryVote, error) {
	digest, err := voteDigest(v)
	if err != nil {
		return SignedAdvisoryVote{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return SignedAdvisoryVote{}, errors.New("invalid Ed25519 private key length")
	}
	// Reconstruct from the seed instead of trusting an inconsistent public half.
	canonicalKey := ed25519.NewKeyFromSeed(key.Seed())
	pub := canonicalKey.Public().(ed25519.PublicKey)
	if !strings.EqualFold(hex.EncodeToString(pub), v.VoterPublicKeyHex) {
		return SignedAdvisoryVote{}, errors.New("signing key does not match voter identity")
	}
	signature := ed25519.Sign(canonicalKey, digest[:])
	return SignedAdvisoryVote{Vote: v, SignatureHex: hex.EncodeToString(signature)}, nil
}

func VerifyAdvisoryVote(s SignedAdvisoryVote, proposal ProposalIntent, trusted Context) error {
	if err := ValidateVoteForProposal(s.Vote, proposal, trusted); err != nil {
		return err
	}
	if len(s.SignatureHex) != ed25519.SignatureSize*2 || strings.ToLower(s.SignatureHex) != s.SignatureHex {
		return errors.New("invalid lowercase advisory vote signature")
	}
	signature, err := hex.DecodeString(s.SignatureHex)
	if err != nil {
		return fmt.Errorf("vote signature encoding: %w", err)
	}
	pub, err := hex.DecodeString(s.Vote.VoterPublicKeyHex)
	if err != nil {
		return fmt.Errorf("vote public key: %w", err)
	}
	digest, err := voteDigest(s.Vote)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), digest[:], signature) {
		return errors.New("invalid advisory vote signature")
	}
	return nil
}

// VoteReplayKey is stable per (network, chain, genesis, proposal, voter), not
// per nonce or choice, so a second vote is a duplicate, not an amendment.
// A durable, atomic reservation is needed to reject duplicates in practice.
func VoteReplayKey(v AdvisoryVoteIntent) ([32]byte, error) {
	if err := ValidateVoteShape(v); err != nil {
		return [32]byte{}, err
	}
	h := sha256.New()
	h.Write([]byte(voteReplayDomain))
	writeString(h, v.Network)
	writeString(h, v.ChainID)
	writeString(h, v.GenesisSHA256)
	writeString(h, v.ProposalSHA256)
	writeString(h, v.VoterPublicKeyHex)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}
