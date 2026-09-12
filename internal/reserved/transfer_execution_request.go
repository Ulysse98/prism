package reserved

import "fmt"

// ReservedTransferExecution requests execution of a previously queued
// ReservedTransferProposal.
//
// ExecutionHeight is deliberately not stored here. The canonical execution
// height is the height of the block containing this object.
type ReservedTransferExecution struct {
	ProposalID string `json:"proposal_id"`
}

func NewReservedTransferExecution(
	proposalID string,
) ReservedTransferExecution {
	return ReservedTransferExecution{
		ProposalID: proposalID,
	}
}

func ValidateReservedTransferExecution(
	execution ReservedTransferExecution,
) error {
	if execution.ProposalID == "" {
		return fmt.Errorf(
			"reserved transfer execution proposal ID cannot be empty",
		)
	}

	return nil
}
