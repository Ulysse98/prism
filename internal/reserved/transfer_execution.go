package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

// ExecuteReservedTransferProposal executes one queued reserved transfer.
//
// Execution is atomic from the reserved package's point of view:
// the pending proposal is removed only after authorization, timelock,
// nonce ordering and reserved accounting have all succeeded.
func (state *GovernanceState) ExecuteReservedTransferProposal(
	proposalID string,
	expectedChainID string,
	currentHeight uint64,
	accounting *AccountingState,
	supplyPolicy consensus.SupplyPolicy,
) (
	ReservedTransferProposal,
	error,
) {
	if state == nil {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved governance state cannot be nil",
			)
	}

	if accounting == nil {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved accounting state cannot be nil",
			)
	}

	if proposalID == "" {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal ID cannot be empty",
			)
	}

	if expectedChainID == "" {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"expected chain ID cannot be empty",
			)
	}

	if state.ChainID != "" &&
		expectedChainID != state.ChainID {

		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved governance chain ID mismatch",
			)
	}

	if state.PendingTransfers == nil {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal is not pending",
			)
	}

	pending, exists :=
		state.PendingTransfers[proposalID]

	if !exists {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal is not pending",
			)
	}

	if err :=
		ValidatePendingReservedTransferProposal(
			pending,
		); err != nil {

		return ReservedTransferProposal{},
			fmt.Errorf(
				"invalid pending reserved transfer proposal: %w",
				err,
			)
	}

	proposal :=
		pending.Proposal

	if proposal.ChainID != expectedChainID {
		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal chain ID mismatch",
			)
	}

	if err :=
		pending.RequireExecutableAt(
			currentHeight,
		); err != nil {

		return ReservedTransferProposal{},
			err
	}

	// Pending transfers within a pool execute strictly in nonce order.
	for otherID, other := range state.PendingTransfers {

		if otherID == proposalID {
			continue
		}

		if other.Proposal.Pool !=
			proposal.Pool {

			continue
		}

		if other.Proposal.Nonce <
			proposal.Nonce {

			return ReservedTransferProposal{},
				fmt.Errorf(
					"reserved transfer proposal cannot execute before earlier pending nonce: nonce=%d earlier=%d",
					proposal.Nonce,
					other.Proposal.Nonce,
				)
		}
	}

	// AcceptReservedTransfer deliberately validates against the
	// currently active authority policy, not the policy that existed
	// when the proposal was queued.
	if err :=
		accounting.AcceptReservedTransfer(
			proposal,
			state.CurrentPolicy,
			expectedChainID,
			supplyPolicy,
		); err != nil {

		return ReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal execution failed: %w",
				err,
			)
	}

	delete(
		state.PendingTransfers,
		proposalID,
	)

	return cloneReservedTransferProposal(
			proposal,
		),
		nil
}
