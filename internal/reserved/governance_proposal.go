package reserved

import "fmt"

func cloneAuthorityProposal(
	proposal AuthorityProposal,
) AuthorityProposal {
	cloned := proposal

	cloned.Change = proposal.Change

	cloned.Change.Approvals =
		append(
			[]Approval(nil),
			proposal.Change.Approvals...,
		)

	return cloned
}

// QueueAuthorityProposal validates an approved governance proposal against
// the currently active authority policy and records it as pending.
//
// Queueing does not mutate CurrentPolicy and does not consume the authority
// change replay state. Replay state is consumed only when the proposal is
// eventually executed.
func (state *GovernanceState) QueueAuthorityProposal(
	proposal AuthorityProposal,
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
		ValidateAuthorityProposal(
			proposal,
		); err != nil {

		return err
	}

	// The active authority policy must authorize the proposal at queue time.
	if err :=
		state.CurrentPolicy.ValidateAuthorityChange(
			proposal.Change,
			expectedChainID,
		); err != nil {

		return fmt.Errorf(
			"reserved authority proposal authorization failed: %w",
			err,
		)
	}

	if state.Replay == nil {
		state.Replay =
			NewReplayState()
	}

	// Reject changes already executed or stale relative to executed
	// governance state, but do not consume replay protection yet.
	if err :=
		state.Replay.ValidateAuthorityChangeNext(
			proposal.Change,
		); err != nil {

		return fmt.Errorf(
			"reserved authority proposal replay validation failed: %w",
			err,
		)
	}

	if state.PendingProposals == nil {
		state.PendingProposals =
			make(
				map[string]PendingAuthorityProposal,
			)
	}

	if _, exists :=
		state.PendingProposals[proposal.ID]; exists {

		return fmt.Errorf(
			"reserved authority proposal already pending",
		)
	}

	// Within each reserved pool, queued proposal nonces must increase.
	// This prevents multiple pending proposals from racing with stale or
	// duplicate authority-change nonces.
	for _, existing := range state.PendingProposals {

		if existing.Proposal.Change.Pool !=
			proposal.Change.Pool {

			continue
		}

		if proposal.Change.Nonce <=
			existing.Proposal.Change.Nonce {

			return fmt.Errorf(
				"reserved authority proposal nonce is not increasing: nonce=%d pending=%d",
				proposal.Change.Nonce,
				existing.Proposal.Change.Nonce,
			)
		}
	}

	cloned :=
		cloneAuthorityProposal(
			proposal,
		)

	pending, err :=
		NewPendingAuthorityProposal(
			cloned,
			proposalHeight,
			delayBlocks,
		)

	if err != nil {
		return err
	}

	state.PendingProposals[proposal.ID] =
		pending

	return nil
}

func (state *GovernanceState) GetPendingAuthorityProposal(
	proposalID string,
) (
	PendingAuthorityProposal,
	bool,
) {
	if state == nil ||
		state.PendingProposals == nil {

		return PendingAuthorityProposal{},
			false
	}

	pending, exists :=
		state.PendingProposals[proposalID]

	if !exists {
		return PendingAuthorityProposal{},
			false
	}

	pending.Proposal =
		cloneAuthorityProposal(
			pending.Proposal,
		)

	return pending, true
}
