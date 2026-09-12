package reserved

import "fmt"

// ValidateReservedTransferProposal validates a signed reserved transfer
// proposal against the currently active authority policy.
//
// The current authorities of the proposal's target pool authorize the
// transfer. Queue-time snapshots will later preserve that authority set
// throughout the proposal lifecycle.
func (policy AuthorityPolicy) ValidateReservedTransferProposal(
	proposal ReservedTransferProposal,
	expectedChainID string,
) error {
	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err != nil {

		return err
	}

	if expectedChainID == "" {
		return fmt.Errorf(
			"expected chain ID cannot be empty",
		)
	}

	if proposal.ChainID != expectedChainID {
		return fmt.Errorf(
			"reserved transfer proposal chain ID mismatch",
		)
	}

	if err := policy.Validate(); err != nil {
		return err
	}

	authorities, err :=
		policy.authorities(
			proposal.Pool,
		)

	if err != nil {
		return err
	}

	threshold, err :=
		policy.EffectiveThreshold(
			proposal.Pool,
		)

	if err != nil {
		return err
	}

	allowed :=
		make(
			map[string]struct{},
			len(authorities),
		)

	for _, authority := range authorities {

		allowed[authority] =
			struct{}{}
	}

	approved :=
		make(
			map[string]struct{},
		)

	for _, approval := range proposal.Approvals {

		if err :=
			ValidateReservedTransferProposalApproval(
				proposal,
				approval,
			); err != nil {

			return err
		}

		if _, exists :=
			allowed[approval.Authorizer]; !exists {

			return fmt.Errorf(
				"reserved transfer proposal approver is not authorized for reserved pool",
			)
		}

		if _, exists :=
			approved[approval.Authorizer]; exists {

			return fmt.Errorf(
				"duplicate reserved transfer proposal approver",
			)
		}

		approved[approval.Authorizer] =
			struct{}{}
	}

	if uint32(len(approved)) <
		threshold {

		return fmt.Errorf(
			"reserved transfer proposal approval threshold not met: approvals=%d threshold=%d",
			len(approved),
			threshold,
		)
	}

	return nil
}
