package reserved

import "fmt"

// ExecuteAuthorityProposal executes one queued authority proposal.
//
// Execution rules:
//   - the proposal must exist in PendingProposals;
//   - its timelock must have elapsed;
//   - no lower pending nonce may exist for the same reserved pool;
//   - the authority change must still be valid against the active policy;
//   - replay protection is consumed only after successful execution.
//
// On success the proposal is removed from PendingProposals.
func (state *GovernanceState) ExecuteAuthorityProposal(
	proposalID string,
	expectedChainID string,
	currentHeight uint64,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved governance state cannot be nil",
		)
	}

	if proposalID == "" {
		return fmt.Errorf(
			"authority proposal ID cannot be empty",
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

	if state.PendingProposals == nil {
		return fmt.Errorf(
			"reserved authority proposal is not pending",
		)
	}

	pending, exists :=
		state.PendingProposals[proposalID]

	if !exists {
		return fmt.Errorf(
			"reserved authority proposal is not pending",
		)
	}

	if err :=
		ValidatePendingAuthorityProposal(
			pending,
		); err != nil {

		return fmt.Errorf(
			"invalid pending authority proposal: %w",
			err,
		)
	}

	change :=
		pending.Proposal.Change

	if change.ChainID != expectedChainID {
		return fmt.Errorf(
			"reserved authority proposal chain ID mismatch",
		)
	}

	if err :=
		pending.RequireExecutableAt(
			currentHeight,
		); err != nil {

		return err
	}

	// Pending proposals within a pool execute in nonce order.
	// A later proposal cannot jump ahead and make an earlier nonce stale.
	for otherID, other := range state.PendingProposals {

		if otherID == proposalID {
			continue
		}

		otherChange :=
			other.Proposal.Change

		if otherChange.Pool != change.Pool {
			continue
		}

		if otherChange.Nonce < change.Nonce {
			return fmt.Errorf(
				"reserved authority proposal cannot execute before earlier pending nonce: nonce=%d earlier=%d",
				change.Nonce,
				otherChange.Nonce,
			)
		}
	}

	// ApplyAuthorityChange performs policy validation and only consumes
	// replay protection after successful authority-set mutation.
	if err :=
		state.ApplyAuthorityChange(
			change,
			expectedChainID,
		); err != nil {

		return fmt.Errorf(
			"reserved authority proposal execution failed: %w",
			err,
		)
	}

	delete(
		state.PendingProposals,
		proposalID,
	)

	return nil
}
