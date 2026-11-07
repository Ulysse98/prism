package governancev069

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
)

const (
	AdvisoryTallyReceiptVersion = "PRISM_GOV_V069_ADVISORY_TALLY_RECEIPT_V1"
	ballotSetDomain             = "PRISM-GOV-v0.69/ADVISORY-BALLOT-SET/v1\x00"
	tallyReceiptDomain          = "PRISM-GOV-v0.69/ADVISORY-TALLY-RECEIPT/v1\x00"
)

// AdvisoryTallyReceipt is a reproducible, unsigned observation of a supplied
// ballot set at one trusted height. It is NOT a finalization certificate,
// authorization, proof of ballot completeness, or on-chain governance result.
// Receipts must always be independently checked against the source ballots,
// proposal, trusted roster pin, independently pinned ballot set and context.
type AdvisoryTallyReceipt struct {
	Version          string        `json:"version"`
	Network          string        `json:"network"`
	ChainID          string        `json:"chain_id"`
	GenesisSHA256    string        `json:"genesis_sha256"`
	ProposalSHA256   string        `json:"proposal_sha256"`
	SnapshotSHA256   string        `json:"snapshot_sha256"`
	BallotSetSHA256  string        `json:"ballot_set_sha256"`
	ObservedAtHeight uint64        `json:"observed_at_height"`
	QuorumBps        uint16        `json:"quorum_bps"`
	ApprovalBps      uint16        `json:"approval_bps"`
	Tally            AdvisoryTally `json:"tally"`
	ReceiptSHA256    string        `json:"receipt_sha256"`
}

// AdvisoryBallotSetCommitment hashes a CANONICAL order of signed vote records,
// including the raw signature bytes. It checks shape and duplicates but DOES
// NOT authenticate signatures or establish ballot completeness. The caller
// must obtain an independently trusted digest pin, not derive and trust its
// own pin from arbitrary ballots. Empty sets are allowed (zero participation).
func AdvisoryBallotSetCommitment(signed []SignedAdvisoryVote) ([32]byte, error) {
	var zero [32]byte
	if len(signed) > MaxAdvisoryVoters {
		return zero, errors.New("ballot set exceeds maximum advisory voter count")
	}
	ordered := append([]SignedAdvisoryVote(nil), signed...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Vote.VoterPublicKeyHex < ordered[j].Vote.VoterPublicKeyHex
	})
	h := sha256.New()
	h.Write([]byte(ballotSetDomain))
	writeUint64(h, uint64(len(ordered)))
	previous := ""
	for _, ballot := range ordered {
		if err := ValidateVoteShape(ballot.Vote); err != nil {
			return zero, fmt.Errorf("invalid ballot shape: %w", err)
		}
		pub := ballot.Vote.VoterPublicKeyHex
		if previous == pub {
			return zero, errors.New("duplicate voter in ballot set")
		}
		previous = pub
		if len(ballot.SignatureHex) != ed25519.SignatureSize*2 || !isLowerHex(ballot.SignatureHex) {
			return zero, errors.New("noncanonical ballot signature encoding")
		}
		signature, err := hex.DecodeString(ballot.SignatureHex)
		if err != nil {
			return zero, fmt.Errorf("ballot signature: %w", err)
		}
		voteHash, err := AdvisoryVoteCommitment(ballot.Vote)
		if err != nil {
			return zero, err
		}
		writeString(h, pub)
		h.Write(voteHash[:])
		h.Write(signature)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func isLowerHex(s string) bool {
	for _, b := range []byte(s) {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}
	return true
}

// BuildAdvisoryTallyReceipt verifies every signature, member, snapshot pin,
// network context and ballot-set pin before returning a stable observation.
// The two pins MUST come from independently reviewed/trusted sources. This
// function never closes voting and does not reconcile the local replay store.
func BuildAdvisoryTallyReceipt(snapshot EligibilitySnapshot, trustedSnapshotSHA256 string, trustedBallotSetSHA256 string, proposal ProposalIntent, trusted Context, ballots []SignedAdvisoryVote) (AdvisoryTallyReceipt, error) {
	var zero AdvisoryTallyReceipt
	if !isLowerSHA256(trustedBallotSetSHA256) {
		return zero, errors.New("missing or malformed trusted ballot-set digest pin")
	}
	result, err := TallyAdvisoryVotes(snapshot, trustedSnapshotSHA256, proposal, trusted, ballots)
	if err != nil {
		return zero, fmt.Errorf("advisory tally: %w", err)
	}
	ballotsHash, err := AdvisoryBallotSetCommitment(ballots)
	if err != nil {
		return zero, fmt.Errorf("ballot-set commitment: %w", err)
	}
	expected, _ := hex.DecodeString(trustedBallotSetSHA256)
	if subtle.ConstantTimeCompare(ballotsHash[:], expected) != 1 {
		return zero, errors.New("trusted ballot-set pin mismatch")
	}
	receipt := AdvisoryTallyReceipt{
		Version: AdvisoryTallyReceiptVersion, Network: trusted.Network,
		ChainID: trusted.ChainID, GenesisSHA256: trusted.GenesisSHA256,
		ProposalSHA256:   result.ProposalSHA256,
		SnapshotSHA256:   result.SnapshotSHA256,
		BallotSetSHA256:  trustedBallotSetSHA256,
		ObservedAtHeight: trusted.CurrentHeight,
		QuorumBps:        snapshot.QuorumBps, ApprovalBps: snapshot.ApprovalBps,
		Tally: result,
	}
	digest := tallyReceiptCommitment(receipt)
	receipt.ReceiptSHA256 = hex.EncodeToString(digest[:])
	return receipt, nil
}

// VerifyAdvisoryTallyReceipt recomputes the entire receipt from full ballots
// and independent pins. A matching SHA-256 alone never authenticates the pins
// or proves that missing ballots were not withheld by the collector.
func VerifyAdvisoryTallyReceipt(receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, trustedSnapshotSHA256 string, trustedBallotSetSHA256 string, proposal ProposalIntent, trusted Context, ballots []SignedAdvisoryVote) error {
	recomputed, err := BuildAdvisoryTallyReceipt(snapshot, trustedSnapshotSHA256, trustedBallotSetSHA256, proposal, trusted, ballots)
	if err != nil {
		return err
	}
	if receipt != recomputed {
		return errors.New("advisory receipt does not match independently recomputed results")
	}
	return nil
}

// tallyReceiptCommitment is only called on a freshly generated, validated
// receipt. All fields are explicitly serialized without JSON or floats.
func tallyReceiptCommitment(r AdvisoryTallyReceipt) [32]byte {
	h := sha256.New()
	h.Write([]byte(tallyReceiptDomain))
	writeString(h, r.Version)
	writeString(h, r.Network)
	writeString(h, r.ChainID)
	writeString(h, r.GenesisSHA256)
	writeString(h, r.ProposalSHA256)
	writeString(h, r.SnapshotSHA256)
	writeString(h, r.BallotSetSHA256)
	writeUint64(h, r.ObservedAtHeight)
	writeUint64(h, uint64(r.QuorumBps))
	writeUint64(h, uint64(r.ApprovalBps))
	writeString(h, r.Tally.SnapshotSHA256)
	writeString(h, r.Tally.ProposalSHA256)
	writeUint64(h, r.Tally.EligibleWeight)
	writeUint64(h, r.Tally.PresentWeight)
	writeUint64(h, r.Tally.YesWeight)
	writeUint64(h, r.Tally.NoWeight)
	writeUint64(h, r.Tally.AbstainWeight)
	writeUint64(h, r.Tally.Participants)
	writeReceiptBool(h, r.Tally.QuorumMet)
	writeReceiptBool(h, r.Tally.ApprovalMet)
	writeReceiptBool(h, r.Tally.AdvisoryPassed)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func writeReceiptBool(h interface{ Write([]byte) (int, error) }, b bool) {
	if b {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
}
