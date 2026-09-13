package reserved

import "fmt"

// PendingAuthorityProposal is the consensus-derived lifecycle state of an
// AuthorityProposal after it has been included on-chain.
//
// ProposalHeight is the actual block height where the proposal entered the
// governance queue.
//
// DelayBlocks is frozen at queue time so later governance configuration
// changes cannot retroactively shorten or extend an existing proposal.
//
// ExecuteAfterHeight is deterministically derived as:
//
//	ProposalHeight + DelayBlocks
type PendingAuthorityProposal struct {
	Proposal AuthorityProposal `json:"proposal"`

	ProposalHeight     uint64 `json:"proposal_height"`
	DelayBlocks        uint64 `json:"delay_blocks"`
	ExecuteAfterHeight uint64 `json:"execute_after_height"`
}

// NewPendingAuthorityProposal converts an approved AuthorityProposal into
// queue state using consensus-derived block height and delay.
//
// This function does not validate the authority approval threshold. That
// belongs to GovernanceState because threshold validation depends on the
// currently active authority policy.
func NewPendingAuthorityProposal(
	proposal AuthorityProposal,
	proposalHeight uint64,
	delayBlocks uint64,
) (
	PendingAuthorityProposal,
	error,
) {
	if err := ValidateAuthorityProposal(
		proposal,
	); err != nil {
		return PendingAuthorityProposal{},
			fmt.Errorf(
				"invalid queued authority proposal: %w",
				err,
			)
	}

	if proposalHeight == 0 {
		return PendingAuthorityProposal{},
			fmt.Errorf(
				"authority proposal height must be greater than zero",
			)
	}

	if delayBlocks == 0 {
		return PendingAuthorityProposal{},
			fmt.Errorf(
				"authority proposal governance delay must be greater than zero",
			)
	}

	if delayBlocks >
		^uint64(0)-proposalHeight {

		return PendingAuthorityProposal{},
			fmt.Errorf(
				"authority proposal execution height overflow",
			)
	}

	pending := PendingAuthorityProposal{
		Proposal:           proposal,
		ProposalHeight:     proposalHeight,
		DelayBlocks:        delayBlocks,
		ExecuteAfterHeight: proposalHeight + delayBlocks,
	}

	if err := ValidatePendingAuthorityProposal(
		pending,
	); err != nil {
		return PendingAuthorityProposal{},
			err
	}

	return pending, nil
}

func ValidatePendingAuthorityProposal(
	pending PendingAuthorityProposal,
) error {
	if err := ValidateAuthorityProposal(
		pending.Proposal,
	); err != nil {
		return fmt.Errorf(
			"invalid pending authority proposal: %w",
			err,
		)
	}

	if pending.ProposalHeight == 0 {
		return fmt.Errorf(
			"pending authority proposal height must be greater than zero",
		)
	}

	if pending.DelayBlocks == 0 {
		return fmt.Errorf(
			"pending authority proposal delay must be greater than zero",
		)
	}

	if pending.DelayBlocks >
		^uint64(0)-pending.ProposalHeight {

		return fmt.Errorf(
			"pending authority proposal execution height overflow",
		)
	}

	expectedExecuteAfter :=
		pending.ProposalHeight +
			pending.DelayBlocks

	if pending.ExecuteAfterHeight !=
		expectedExecuteAfter {

		return fmt.Errorf(
			"invalid pending authority proposal execution height: got=%d expected=%d",
			pending.ExecuteAfterHeight,
			expectedExecuteAfter,
		)
	}

	return nil
}

// IsExecutableAt reports whether the proposal has reached its minimum
// execution height.
//
// This is only a lifecycle timing check. Actual execution must still verify
// that the proposal exists, has not already executed or been cancelled, and
// satisfies all governance-state rules.
func (
	pending PendingAuthorityProposal,
) IsExecutableAt(
	height uint64,
) bool {
	return height >=
		pending.ExecuteAfterHeight
}

// RequireExecutableAt validates the pending proposal and rejects execution
// before its deterministic timelock boundary.
func (
	pending PendingAuthorityProposal,
) RequireExecutableAt(
	height uint64,
) error {
	if err := ValidatePendingAuthorityProposal(
		pending,
	); err != nil {
		return err
	}

	if height <
		pending.ExecuteAfterHeight {

		return fmt.Errorf(
			"authority proposal timelock not reached: current=%d execute_after=%d",
			height,
			pending.ExecuteAfterHeight,
		)
	}

	return nil
}
