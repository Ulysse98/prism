package reserved

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"prism/internal/consensus"
)

// ReservedTransferProposal represents an intent to transfer funds
// from one of Prism's reserved pools.
//
// Execution timing is deliberately not stored in the proposal.
// The queued-governance lifecycle derives execution height from
// consensus rules when the proposal is included on-chain.
type ReservedTransferProposal struct {
	ID string `json:"id"`

	ChainID string `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`

	Pool      consensus.ReservedPool `json:"pool"`
	Recipient string                 `json:"recipient"`
	Amount    uint64                 `json:"amount"`

	Approvals []Approval `json:"approvals,omitempty"`
}

func NewReservedTransferProposal(
	chainID string,
	nonce uint64,
	pool consensus.ReservedPool,
	recipient string,
	amount uint64,
) ReservedTransferProposal {
	proposal := ReservedTransferProposal{
		ChainID:   strings.TrimSpace(chainID),
		Nonce:     nonce,
		Pool:      pool,
		Recipient: strings.TrimSpace(recipient),
		Amount:    amount,
		Approvals: []Approval{},
	}

	proposal.ID =
		CalculateReservedTransferProposalID(
			proposal,
		)

	return proposal
}

func reservedTransferProposalPayload(
	proposal ReservedTransferProposal,
) string {
	return fmt.Sprintf(
		"reserved-transfer-proposal-v1|%s|%d|%s|%s|%d",
		proposal.ChainID,
		proposal.Nonce,
		proposal.Pool,
		proposal.Recipient,
		proposal.Amount,
	)
}

func CalculateReservedTransferProposalID(
	proposal ReservedTransferProposal,
) string {
	hash := sha256.Sum256(
		[]byte(
			reservedTransferProposalPayload(
				proposal,
			),
		),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

func ValidateReservedTransferProposal(
	proposal ReservedTransferProposal,
) error {
	if proposal.ChainID == "" {
		return fmt.Errorf(
			"reserved transfer proposal chain ID cannot be empty",
		)
	}

	if proposal.Nonce == 0 {
		return fmt.Errorf(
			"reserved transfer proposal nonce must be greater than zero",
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	allocation, err :=
		supplyPolicy.ReservedPoolAllocation(
			proposal.Pool,
		)

	if err != nil {
		return err
	}

	if proposal.Recipient == "" {
		return fmt.Errorf(
			"reserved transfer proposal recipient cannot be empty",
		)
	}

	if proposal.Recipient == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot receive a reserved transfer",
		)
	}

	if proposal.Amount == 0 {
		return fmt.Errorf(
			"reserved transfer proposal amount must be greater than zero",
		)
	}

	if proposal.Amount > allocation {
		return fmt.Errorf(
			"reserved transfer proposal amount exceeds pool allocation",
		)
	}

	if proposal.ID == "" {
		return fmt.Errorf(
			"reserved transfer proposal ID cannot be empty",
		)
	}

	if proposal.ID !=
		CalculateReservedTransferProposalID(
			proposal,
		) {

		return fmt.Errorf(
			"invalid reserved transfer proposal ID",
		)
	}

	return nil
}
