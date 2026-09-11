package reserved

import "fmt"

// AuthorityExecution requests execution of a previously queued
// AuthorityProposal.
//
// ExecutionHeight is deliberately not stored here. The canonical execution
// height is the height of the block containing this object.
type AuthorityExecution struct {
	ProposalID string `json:"proposal_id"`
}

func NewAuthorityExecution(
	proposalID string,
) AuthorityExecution {
	return AuthorityExecution{
		ProposalID: proposalID,
	}
}

func ValidateAuthorityExecution(
	execution AuthorityExecution,
) error {
	if execution.ProposalID == "" {
		return fmt.Errorf(
			"authority execution proposal ID cannot be empty",
		)
	}

	return nil
}