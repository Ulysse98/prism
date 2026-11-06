package governancev069

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
)

const (
	EligibilitySnapshotVersion  = "PRISM_GOV_V069_ELIGIBILITY_SNAPSHOT_V1"
	eligibilityCommitmentDomain = "PRISM-GOV-v0.69/ELIGIBILITY-SNAPSHOT/v1\x00"
	MaxAdvisoryVoters           = 10000
)

// EligibleVoter identifies one *declared* voter. No token, staking or
// validator status is implied by membership in this list.
type EligibleVoter struct {
	PublicKeyHex string `json:"public_key_hex"`
	Weight       uint64 `json:"weight"`
}

// EligibilitySnapshot is a pre-agreed advisory roster. A digest pin binds
// bytes but does NOT authenticate the authority that selected the roster.
// Members must be sorted strictly ascending by public key for canonicality.
type EligibilitySnapshot struct {
	Version        string          `json:"version"`
	Network        string          `json:"network"`
	ChainID        string          `json:"chain_id"`
	GenesisSHA256  string          `json:"genesis_sha256"`
	ProposalSHA256 string          `json:"proposal_sha256"`
	SnapshotHeight uint64          `json:"snapshot_height"`
	QuorumBps      uint16          `json:"quorum_bps"`
	ApprovalBps    uint16          `json:"approval_bps"`
	Members        []EligibleVoter `json:"members"`
}

// ValidateEligibilitySnapshot rejects ambiguous order, duplicate IDs,
// arithmetic overflow, and any absent/unsupported advisory threshold policy.
func ValidateEligibilitySnapshot(s EligibilitySnapshot) error {
	if s.Version != EligibilitySnapshotVersion {
		return errors.New("invalid eligibility snapshot version")
	}
	if !asciiIdentifier(s.Network, 1, 48) || !asciiIdentifier(s.ChainID, 1, 128) {
		return errors.New("invalid snapshot network or chain")
	}
	if !isLowerSHA256(s.GenesisSHA256) || !isLowerSHA256(s.ProposalSHA256) {
		return errors.New("invalid snapshot genesis or proposal digest")
	}
	if s.SnapshotHeight == 0 {
		return errors.New("snapshot height must be nonzero")
	}
	if s.QuorumBps == 0 || s.QuorumBps > 10000 {
		return errors.New("quorum must be 1..10000 basis points")
	}
	if s.ApprovalBps < 5001 || s.ApprovalBps > 10000 {
		return errors.New("approval must be strict majority or higher")
	}
	if len(s.Members) == 0 || len(s.Members) > MaxAdvisoryVoters {
		return errors.New("invalid eligibility member count")
	}
	var total uint64
	previous := ""
	for _, m := range s.Members {
		if !isLowerSHA256(m.PublicKeyHex) || (previous != "" && m.PublicKeyHex <= previous) {
			return errors.New("members must have distinct, canonical, strictly ordered public keys")
		}
		if m.Weight == 0 || m.Weight > math.MaxUint64-total {
			return errors.New("zero member weight or total weight overflow")
		}
		total += m.Weight
		previous = m.PublicKeyHex
	}
	return nil
}

// EligibilitySnapshotCommitment binds every roster member and both policy
// thresholds using an explicit, domain-separated binary encoding.
func EligibilitySnapshotCommitment(s EligibilitySnapshot) ([32]byte, error) {
	if err := ValidateEligibilitySnapshot(s); err != nil {
		return [32]byte{}, err
	}
	h := sha256.New()
	h.Write([]byte(eligibilityCommitmentDomain))
	writeString(h, s.Version)
	writeString(h, s.Network)
	writeString(h, s.ChainID)
	writeString(h, s.GenesisSHA256)
	writeString(h, s.ProposalSHA256)
	writeUint64(h, s.SnapshotHeight)
	writeUint64(h, uint64(s.QuorumBps))
	writeUint64(h, uint64(s.ApprovalBps))
	writeUint64(h, uint64(len(s.Members)))
	for _, m := range s.Members {
		writeString(h, m.PublicKeyHex)
		writeUint64(h, m.Weight)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

// AdvisoryTally is a read-only local calculation, never an authorization,
// quorum certificate, validator signature or executable governance decision.
type AdvisoryTally struct {
	SnapshotSHA256 string `json:"snapshot_sha256"`
	ProposalSHA256 string `json:"proposal_sha256"`
	EligibleWeight uint64 `json:"eligible_weight"`
	PresentWeight  uint64 `json:"present_weight"`
	YesWeight      uint64 `json:"yes_weight"`
	NoWeight       uint64 `json:"no_weight"`
	AbstainWeight  uint64 `json:"abstain_weight"`
	Participants   uint64 `json:"participants"`
	QuorumMet      bool   `json:"quorum_met"`
	ApprovalMet    bool   `json:"approval_met"`
	AdvisoryPassed bool   `json:"advisory_passed"`
}

// TallyAdvisoryVotes requires a separately trusted, lower-case pinned roster
// digest. It verifies every signature and fails the *whole* tally on any
// malformed, duplicate or ineligible vote; no partial success on bad input.
// Abstentions count toward quorum but are excluded from the approval denominator.
// The snapshot must have been frozen at the proposal's created_at_height.
func TallyAdvisoryVotes(s EligibilitySnapshot, pinnedSnapshotSHA256 string, proposal ProposalIntent, trusted Context, signed []SignedAdvisoryVote) (AdvisoryTally, error) {
	var empty AdvisoryTally
	if err := ValidateForContext(proposal, trusted); err != nil {
		return empty, fmt.Errorf("proposal context: %w", err)
	}
	snapshotDigest, err := EligibilitySnapshotCommitment(s)
	if err != nil {
		return empty, fmt.Errorf("snapshot: %w", err)
	}
	if !isLowerSHA256(pinnedSnapshotSHA256) {
		return empty, errors.New("missing or malformed trusted snapshot digest pin")
	}
	expected, _ := hex.DecodeString(pinnedSnapshotSHA256)
	if subtle.ConstantTimeCompare(snapshotDigest[:], expected) != 1 {
		return empty, errors.New("trusted eligibility snapshot pin mismatch")
	}
	proposalDigest, err := Commitment(proposal)
	if err != nil {
		return empty, err
	}
	proposalHex := hex.EncodeToString(proposalDigest[:])
	if s.Network != trusted.Network || s.ChainID != trusted.ChainID || s.GenesisSHA256 != trusted.GenesisSHA256 || s.ProposalSHA256 != proposalHex {
		return empty, errors.New("eligibility snapshot disagrees with trusted proposal/network context")
	}
	if s.SnapshotHeight != proposal.CreatedAtHeight || s.SnapshotHeight > trusted.CurrentHeight {
		return empty, errors.New("snapshot must be frozen at proposal creation height")
	}
	if len(signed) > len(s.Members) {
		return empty, errors.New("more vote records than eligible voters")
	}

	weights := make(map[string]uint64, len(s.Members))
	var total uint64
	for _, m := range s.Members {
		weights[m.PublicKeyHex] = m.Weight
		total += m.Weight // validated bounded sum
	}
	result := AdvisoryTally{SnapshotSHA256: pinnedSnapshotSHA256, ProposalSHA256: proposalHex, EligibleWeight: total}
	seen := make(map[string]bool, len(signed))
	for _, v := range signed {
		if err := VerifyAdvisoryVote(v, proposal, trusted); err != nil {
			return empty, fmt.Errorf("invalid signed advisory vote: %w", err)
		}
		key := v.Vote.VoterPublicKeyHex
		weight, ok := weights[key]
		if !ok {
			return empty, errors.New("signed voter not present in eligible roster")
		}
		if seen[key] {
			return empty, errors.New("duplicate voter across advisory votes")
		}
		seen[key] = true
		// There can be no overflow because each weight is counted at most once
		// and the sum of roster weights is <= MaxUint64.
		result.PresentWeight += weight
		result.Participants++
		switch v.Vote.Choice {
		case VoteYes:
			result.YesWeight += weight
		case VoteNo:
			result.NoWeight += weight
		case VoteAbstain:
			result.AbstainWeight += weight
		default:
			return empty, errors.New("invalid advisory choice")
		}
	}
	result.QuorumMet = atLeastBps(result.PresentWeight, result.EligibleWeight, s.QuorumBps)
	expressed := result.YesWeight + result.NoWeight // bounded by roster total
	result.ApprovalMet = expressed != 0 && atLeastBps(result.YesWeight, expressed, s.ApprovalBps)
	result.AdvisoryPassed = result.QuorumMet && result.ApprovalMet
	return result, nil
}

// atLeastBps performs integer-only exact comparisons; big.Int is used to
// prevent overflow for near-MaxUint64 rosters and avoid floating point drift.
func atLeastBps(numerator, denominator uint64, thresholdBps uint16) bool {
	if denominator == 0 {
		return false
	}
	left := new(big.Int).Mul(new(big.Int).SetUint64(numerator), big.NewInt(10000))
	right := new(big.Int).Mul(new(big.Int).SetUint64(denominator), new(big.Int).SetUint64(uint64(thresholdBps)))
	return left.Cmp(right) >= 0
}
