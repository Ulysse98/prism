package governancev069

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

const (
	AdvisoryReservationAuditVersion = "PRISM_GOV_V069_LOCAL_RESERVATION_AUDIT_V1"
	reservationAuditDomain          = "PRISM-GOV-v0.69/LOCAL-RESERVATION-AUDIT/v1\x00"
)

// AdvisoryReservationAudit is an UNSIGNED local filesystem inspection receipt.
// It does not freeze a vote store, prove data availability, or authorize
// production governance, consensus, treasury actions, or execution.
// The caller must independently control the trusted pins and arrange for a
// quiescent, exclusive, one-proposal reservation directory during inspection.
type AdvisoryReservationAudit struct {
	Version             string `json:"version"`
	Network             string `json:"network"`
	ChainID             string `json:"chain_id"`
	GenesisSHA256       string `json:"genesis_sha256"`
	ProposalSHA256      string `json:"proposal_sha256"`
	SnapshotSHA256      string `json:"snapshot_sha256"`
	BallotSetSHA256     string `json:"ballot_set_sha256"`
	TallyReceiptSHA256  string `json:"tally_receipt_sha256"`
	ObservedAtHeight    uint64 `json:"observed_at_height"`
	MatchedReservations uint64 `json:"matched_reservations"`
	AuditSHA256         string `json:"audit_sha256"`
}

func reservationAuditCommitment(a AdvisoryReservationAudit) [32]byte {
	h := sha256.New()
	h.Write([]byte(reservationAuditDomain))
	writeString(h, a.Version)
	writeString(h, a.Network)
	writeString(h, a.ChainID)
	writeString(h, a.GenesisSHA256)
	writeString(h, a.ProposalSHA256)
	writeString(h, a.SnapshotSHA256)
	writeString(h, a.BallotSetSHA256)
	writeString(h, a.TallyReceiptSHA256)
	writeUint64(h, a.ObservedAtHeight)
	writeUint64(h, a.MatchedReservations)
	return finishReservationHash(h)
}

func finishReservationHash(h hash.Hash) [32]byte {
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func makeAdvisoryReservationAudit(receipt AdvisoryTallyReceipt, count int) AdvisoryReservationAudit {
	a := AdvisoryReservationAudit{
		Version: AdvisoryReservationAuditVersion,
		Network: receipt.Network, ChainID: receipt.ChainID,
		GenesisSHA256:       receipt.GenesisSHA256,
		ProposalSHA256:      receipt.ProposalSHA256,
		SnapshotSHA256:      receipt.SnapshotSHA256,
		BallotSetSHA256:     receipt.BallotSetSHA256,
		TallyReceiptSHA256:  receipt.ReceiptSHA256,
		ObservedAtHeight:    receipt.ObservedAtHeight,
		MatchedReservations: uint64(count),
	}
	hash := reservationAuditCommitment(a)
	a.AuditSHA256 = hex.EncodeToString(hash[:])
	return a
}
