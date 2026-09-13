package reserved

import "fmt"

// PendingReservedTransferProposal is the consensus-derived lifecycle
// state of a reserved transfer after it enters the governance queue.
//
// ProposalHeight is the block where the transfer entered the queue.
// DelayBlocks is frozen at queue time.
// ExecuteAfterHeight is derived deterministically:
//
// ProposalHeight + DelayBlocks
type PendingReservedTransferProposal struct {
	Proposal ReservedTransferProposal `json:"proposal"`

	ProposalHeight     uint64 `json:"proposal_height"`
	DelayBlocks        uint64 `json:"delay_blocks"`
	ExecuteAfterHeight uint64 `json:"execute_after_height"`
}

// NewPendingReservedTransferProposal converts a valid transfer proposal
// into deterministic queued lifecycle state.
//
// Authorization quorum is intentionally not checked here because that
// depends on GovernanceState.CurrentPolicy.
func NewPendingReservedTransferProposal(
	proposal ReservedTransferProposal,
	proposalHeight uint64,
	delayBlocks uint64,
) (
	PendingReservedTransferProposal,
	error,
) {
	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err != nil {

		return PendingReservedTransferProposal{},
			fmt.Errorf(
				"invalid queued reserved transfer proposal: %w",
				err,
			)
	}

	if proposalHeight == 0 {
		return PendingReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal height must be greater than zero",
			)
	}

	if delayBlocks == 0 {
		return PendingReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal governance delay must be greater than zero",
			)
	}

	if delayBlocks >
		^uint64(0)-proposalHeight {

		return PendingReservedTransferProposal{},
			fmt.Errorf(
				"reserved transfer proposal execution height overflow",
			)
	}

	pending :=
		PendingReservedTransferProposal{
			Proposal:           proposal,
			ProposalHeight:     proposalHeight,
			DelayBlocks:        delayBlocks,
			ExecuteAfterHeight: proposalHeight + delayBlocks,
		}

	if err :=
		ValidatePendingReservedTransferProposal(
			pending,
		); err != nil {

		return PendingReservedTransferProposal{},
			err
	}

	return pending, nil
}

func ValidatePendingReservedTransferProposal(
	pending PendingReservedTransferProposal,
) error {
	if err :=
		ValidateReservedTransferProposal(
			pending.Proposal,
		); err != nil {

		return fmt.Errorf(
			"invalid pending reserved transfer proposal: %w",
			err,
		)
	}

	if pending.ProposalHeight == 0 {
		return fmt.Errorf(
			"pending reserved transfer proposal height must be greater than zero",
		)
	}

	if pending.DelayBlocks == 0 {
		return fmt.Errorf(
			"pending reserved transfer proposal delay must be greater than zero",
		)
	}

	if pending.DelayBlocks >
		^uint64(0)-pending.ProposalHeight {

		return fmt.Errorf(
			"pending reserved transfer proposal execution height overflow",
		)
	}

	expectedExecuteAfter :=
		pending.ProposalHeight +
			pending.DelayBlocks

	if pending.ExecuteAfterHeight !=
		expectedExecuteAfter {

		return fmt.Errorf(
			"invalid pending reserved transfer proposal execution height: got=%d expected=%d",
			pending.ExecuteAfterHeight,
			expectedExecuteAfter,
		)
	}

	return nil
}

func (
	pending PendingReservedTransferProposal,
) IsExecutableAt(
	height uint64,
) bool {
	return height >=
		pending.ExecuteAfterHeight
}

func (
	pending PendingReservedTransferProposal,
) RequireExecutableAt(
	height uint64,
) error {
	if err :=
		ValidatePendingReservedTransferProposal(
			pending,
		); err != nil {

		return err
	}

	if height <
		pending.ExecuteAfterHeight {

		return fmt.Errorf(
			"reserved transfer proposal timelock not reached: current=%d execute_after=%d",
			height,
			pending.ExecuteAfterHeight,
		)
	}

	return nil
}
