package reserved

import "fmt"

func cloneReservedTransferProposal(
	proposal ReservedTransferProposal,
) ReservedTransferProposal {
	cloned := proposal

	cloned.Approvals =
		append(
			[]Approval(nil),
			proposal.Approvals...,
		)

	return cloned
}

// QueueReservedTransferProposal validates an approved reserved transfer
// against the currently active authority policy and records it as pending.
//
// Queueing freezes the lifecycle timing but does not spend reserved funds.
func (state *GovernanceState) QueueReservedTransferProposal(
	proposal ReservedTransferProposal,
	expectedChainID string,
	proposalHeight uint64,
	delayBlocks uint64,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved governance state cannot be nil",
		)
	}

	if expectedChainID == "" {
		return fmt.Errorf(
			"expected chain ID cannot be empty",
		)
	}

	if state.ChainID != "" &&
		expectedChainID != state.ChainID {

		return fmt.Errorf(
			"reserved governance chain ID mismatch",
		)
	}

	if err :=
		state.CurrentPolicy.ValidateReservedTransferProposal(
			proposal,
			expectedChainID,
		); err != nil {

		return fmt.Errorf(
			"reserved transfer proposal authorization failed: %w",
			err,
		)
	}

	if state.PendingTransfers == nil {
		state.PendingTransfers =
			make(
				map[string]PendingReservedTransferProposal,
			)
	}

	if _, exists :=
		state.PendingTransfers[proposal.ID]; exists {

		return fmt.Errorf(
			"reserved transfer proposal already pending",
		)
	}

	// Pending transfers within the same pool must use increasing nonces.
	for _, existing := range state.PendingTransfers {

		if existing.Proposal.Pool !=
			proposal.Pool {

			continue
		}

		if proposal.Nonce <=
			existing.Proposal.Nonce {

			return fmt.Errorf(
				"reserved transfer proposal nonce is not increasing: nonce=%d pending=%d",
				proposal.Nonce,
				existing.Proposal.Nonce,
			)
		}
	}

	cloned :=
		cloneReservedTransferProposal(
			proposal,
		)

	pending, err :=
		NewPendingReservedTransferProposal(
			cloned,
			proposalHeight,
			delayBlocks,
		)

	if err != nil {
		return err
	}

	state.PendingTransfers[proposal.ID] =
		pending

	return nil
}

func (state *GovernanceState) GetPendingReservedTransferProposal(
	proposalID string,
) (
	PendingReservedTransferProposal,
	bool,
) {
	if state == nil ||
		state.PendingTransfers == nil {

		return PendingReservedTransferProposal{},
			false
	}

	pending, exists :=
		state.PendingTransfers[proposalID]

	if !exists {
		return PendingReservedTransferProposal{},
			false
	}

	pending.Proposal =
		cloneReservedTransferProposal(
			pending.Proposal,
		)

	return pending, true
}
