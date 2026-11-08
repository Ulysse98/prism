package governancev069

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const (
	AdvisoryBallotClosureVersion = "PRISM_GOV_V069_LOCAL_BALLOT_CLOSURE_V1"
	localBallotClosureDomain     = "PRISM-GOV-v0.69/LOCAL-BALLOT-CLOSURE/v1\x00"
)

var ErrAdvisoryBallotClosed = errors.New("local advisory ballot store is sealed or has an ambiguous closure marker")

// AdvisoryBallotClosure is an unsigned, single-host, operator-triggered
// persistent admission gate and inspection receipt. It is NOT a signed
// election certification, distributed finality, or execution authority.
type AdvisoryBallotClosure struct {
	Version                string `json:"version"`
	Network                string `json:"network"`
	ChainID                string `json:"chain_id"`
	GenesisSHA256          string `json:"genesis_sha256"`
	ProposalSHA256         string `json:"proposal_sha256"`
	SnapshotSHA256         string `json:"snapshot_sha256"`
	BallotSetSHA256        string `json:"ballot_set_sha256"`
	TallyReceiptSHA256     string `json:"tally_receipt_sha256"`
	ReservationAuditSHA256 string `json:"reservation_audit_sha256"`
	ClosedAtHeight         uint64 `json:"closed_at_height"`
	ClosureSHA256          string `json:"closure_sha256"`
}

func makeLocalBallotClosure(receipt AdvisoryTallyReceipt, audit AdvisoryReservationAudit) AdvisoryBallotClosure {
	c := AdvisoryBallotClosure{
		Version:                AdvisoryBallotClosureVersion,
		Network:                receipt.Network,
		ChainID:                receipt.ChainID,
		GenesisSHA256:          receipt.GenesisSHA256,
		ProposalSHA256:         receipt.ProposalSHA256,
		SnapshotSHA256:         receipt.SnapshotSHA256,
		BallotSetSHA256:        receipt.BallotSetSHA256,
		TallyReceiptSHA256:     receipt.ReceiptSHA256,
		ReservationAuditSHA256: audit.AuditSHA256,
		ClosedAtHeight:         receipt.ObservedAtHeight,
	}
	digest := localBallotClosureCommitment(c)
	c.ClosureSHA256 = hex.EncodeToString(digest[:])
	return c
}

func localBallotClosureCommitment(c AdvisoryBallotClosure) [32]byte {
	h := sha256.New()
	h.Write([]byte(localBallotClosureDomain))
	writeString(h, c.Version)
	writeString(h, c.Network)
	writeString(h, c.ChainID)
	writeString(h, c.GenesisSHA256)
	writeString(h, c.ProposalSHA256)
	writeString(h, c.SnapshotSHA256)
	writeString(h, c.BallotSetSHA256)
	writeString(h, c.TallyReceiptSHA256)
	writeString(h, c.ReservationAuditSHA256)
	writeUint64(h, c.ClosedAtHeight)
	return finishReservationHash(h)
}
