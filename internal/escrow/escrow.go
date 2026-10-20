// Package escrow models a deterministic escrow lifecycle.
// It does NOT reserve or transfer chain balances. Consensus integration must
// authenticate funding transactions and independently verify PoUW proofs.
package escrow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type Status string

const (
	Funded   Status = "FUNDED"
	Claimed  Status = "CLAIMED"
	Released Status = "RELEASED"
	Refunded Status = "REFUNDED"
)

// Funding describes a canonical, already-validated on-chain funding event.
// Authenticating the transaction and preventing reuse across jobs is a future
// consensus responsibility, not something this in-memory model can guarantee.
type Funding struct {
	ChainID         string
	GenesisHash     string
	JobID           string
	FundingTxID     string
	Requester       string
	Amount          uint64
	FundedAtHeight  uint64
	ExpiresAtHeight uint64
}

// ProofReference is a reference to a proof that MUST be independently verified
// by the consensus caller before Release is invoked. It is not an attestation.
type ProofReference struct {
	ID          string
	ChainID     string
	GenesisHash string
	JobID       string
	Worker      string
}

type Escrow struct {
	ID      string  `json:"id"`
	Funding Funding `json:"funding"`
	Status  Status  `json:"status"`
	Worker  string  `json:"worker,omitempty"`
	ProofID string  `json:"proofId,omitempty"`
}

func isHex(value string, bytes int) bool {
	if len(value) != bytes*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validAddress(value string) bool {
	return strings.HasPrefix(value, "prism_") && isHex(strings.TrimPrefix(value, "prism_"), 20)
}

func validateFunding(f Funding) error {
	if !strings.HasPrefix(f.ChainID, "prism-") || !isHex(strings.TrimPrefix(f.ChainID, "prism-"), 8) {
		return errors.New("invalid chain ID")
	}
	if !isHex(f.GenesisHash, 32) || !isHex(f.JobID, 32) || !isHex(f.FundingTxID, 32) {
		return errors.New("invalid genesis, job or funding transaction ID")
	}
	if !validAddress(f.Requester) {
		return errors.New("invalid requester address")
	}
	if f.Amount == 0 || f.FundedAtHeight == 0 || f.ExpiresAtHeight <= f.FundedAtHeight {
		return errors.New("invalid amount or funding/expiry heights")
	}
	return nil
}

func calculateID(f Funding) string {
	payload := fmt.Sprintf("prism-escrow-v1|%s|%s|%s|%s", f.ChainID, f.GenesisHash, f.JobID, f.FundingTxID)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func NewFunded(f Funding) (Escrow, error) {
	if err := validateFunding(f); err != nil {
		return Escrow{}, err
	}
	e := Escrow{ID: calculateID(f), Funding: f, Status: Funded}
	return e, nil
}

func (e Escrow) Validate() error {
	if err := validateFunding(e.Funding); err != nil {
		return err
	}
	if e.ID != calculateID(e.Funding) {
		return errors.New("escrow ID/context mismatch")
	}
	if e.Worker != "" && !validAddress(e.Worker) {
		return errors.New("invalid worker address")
	}
	switch e.Status {
	case Funded:
		if e.Worker != "" || e.ProofID != "" {
			return errors.New("funded escrow contains claim/proof")
		}
	case Claimed:
		if e.Worker == "" || e.ProofID != "" {
			return errors.New("invalid claimed escrow")
		}
	case Released:
		if e.Worker == "" || !isHex(e.ProofID, 32) {
			return errors.New("invalid released escrow")
		}
	case Refunded:
		if e.ProofID != "" {
			return errors.New("refunded escrow contains proof")
		}
	default:
		return errors.New("invalid escrow status")
	}
	return nil
}

func (e *Escrow) Claim(worker string, height uint64) error {
	if e == nil {
		return errors.New("nil escrow")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if e.Status != Funded {
		return errors.New("escrow is not fundable for claim")
	}
	if !validAddress(worker) || worker == e.Funding.Requester {
		return errors.New("invalid or self-claiming worker")
	}
	if height < e.Funding.FundedAtHeight || height >= e.Funding.ExpiresAtHeight {
		return errors.New("claim height outside active window")
	}
	next := *e
	next.Status, next.Worker = Claimed, worker
	if err := next.Validate(); err != nil {
		return err
	}
	*e = next
	return nil
}

// Release only checks state/context binding. The caller must first verify the
// signed, job-bound PoUW proof as part of consensus validation.
func (e *Escrow) Release(proof ProofReference, height uint64) error {
	if e == nil {
		return errors.New("nil escrow")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if e.Status != Claimed {
		return errors.New("escrow is not claimed")
	}
	if height < e.Funding.FundedAtHeight || height >= e.Funding.ExpiresAtHeight {
		return errors.New("release height outside active window")
	}
	if !isHex(proof.ID, 32) || proof.JobID != e.Funding.JobID || proof.Worker != e.Worker || proof.ChainID != e.Funding.ChainID || proof.GenesisHash != e.Funding.GenesisHash {
		return errors.New("proof reference does not match escrow context")
	}
	next := *e
	next.Status, next.ProofID = Released, proof.ID
	if err := next.Validate(); err != nil {
		return err
	}
	*e = next
	return nil
}

// Refund is allowed at expiry, whether or not a worker claimed the job.
// The consensus caller must enforce canonical ordering and signed authority.
func (e *Escrow) Refund(height uint64) error {
	if e == nil {
		return errors.New("nil escrow")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if e.Status != Funded && e.Status != Claimed {
		return errors.New("escrow already finalized")
	}
	if height < e.Funding.ExpiresAtHeight {
		return errors.New("escrow not yet expired")
	}
	next := *e
	next.Status = Refunded
	if err := next.Validate(); err != nil {
		return err
	}
	*e = next
	return nil
}
