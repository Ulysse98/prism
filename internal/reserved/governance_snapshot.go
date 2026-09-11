package reserved

import (
	"fmt"
	"sort"
)

type GovernanceSnapshot struct {
	ChainID       string          `json:"chain_id,omitempty"`
	CurrentPolicy AuthorityPolicy `json:"current_policy"`
	Replay        ReplaySnapshot  `json:"replay"`

	PendingProposals []PendingAuthorityProposal `json:"pending_proposals,omitempty"`
}

func (state *GovernanceState) Snapshot() (
	GovernanceSnapshot,
	error,
) {
	if state == nil {
		return GovernanceSnapshot{},
			fmt.Errorf(
				"reserved governance state cannot be nil",
			)
	}

	if err := state.CurrentPolicy.Validate(); err != nil {
		return GovernanceSnapshot{},
			fmt.Errorf(
				"invalid current reserved authority policy: %w",
				err,
			)
	}

	if state.Replay == nil {
		return GovernanceSnapshot{},
			fmt.Errorf(
				"reserved governance replay state cannot be nil",
			)
	}

	replaySnapshot, err :=
		state.Replay.Snapshot()

	if err != nil {
		return GovernanceSnapshot{}, err
	}

	pendingProposals :=
		make(
			[]PendingAuthorityProposal,
			0,
			len(state.PendingProposals),
		)

	for proposalID, pending := range state.PendingProposals {

		if proposalID !=
			pending.Proposal.ID {

			return GovernanceSnapshot{},
				fmt.Errorf(
					"pending authority proposal map key mismatch",
				)
		}

		if err :=
			ValidatePendingAuthorityProposal(
				pending,
			); err != nil {

			return GovernanceSnapshot{},
				fmt.Errorf(
					"invalid pending authority proposal: %w",
					err,
				)
		}

		cloned := pending

		cloned.Proposal =
			cloneAuthorityProposal(
				pending.Proposal,
			)

		pendingProposals =
			append(
				pendingProposals,
				cloned,
			)
	}

	// Keep snapshot serialization deterministic regardless of Go map order.
	sort.Slice(
		pendingProposals,
		func(
			i int,
			j int,
		) bool {
			return pendingProposals[i].Proposal.ID <
				pendingProposals[j].Proposal.ID
		},
	)

	return GovernanceSnapshot{
		ChainID:          state.ChainID,
		CurrentPolicy:    state.CurrentPolicy,
		Replay:           replaySnapshot,
		PendingProposals: pendingProposals,
	}, nil
}

func GovernanceStateFromSnapshot(
	snapshot GovernanceSnapshot,
) (
	*GovernanceState,
	error,
) {
	if err := snapshot.CurrentPolicy.Validate(); err != nil {
		return nil,
			fmt.Errorf(
				"invalid stored reserved authority policy: %w",
				err,
			)
	}

	replay, err :=
		ReplayStateFromSnapshot(
			snapshot.Replay,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"invalid stored reserved replay state: %w",
				err,
			)
	}

	pendingProposals :=
		make(
			map[string]PendingAuthorityProposal,
			len(snapshot.PendingProposals),
		)

	type poolNonce struct {
		Pool  string
		Nonce uint64
	}

	usedPendingNonces :=
		make(
			map[poolNonce]struct{},
		)

	for _, pending := range snapshot.PendingProposals {

		if err :=
			ValidatePendingAuthorityProposal(
				pending,
			); err != nil {

			return nil,
				fmt.Errorf(
					"invalid stored pending authority proposal: %w",
					err,
				)
		}

		if snapshot.ChainID != "" &&
			pending.Proposal.Change.ChainID !=
				snapshot.ChainID {

			return nil,
				fmt.Errorf(
					"stored pending authority proposal chain ID mismatch",
				)
		}

		proposalID :=
			pending.Proposal.ID

		if _, exists :=
			pendingProposals[proposalID]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored pending authority proposal",
				)
		}

		key := poolNonce{
			Pool: string(
				pending.Proposal.Change.Pool,
			),
			Nonce: pending.Proposal.Change.Nonce,
		}

		if _, exists :=
			usedPendingNonces[key]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored pending authority proposal nonce",
				)
		}

		usedPendingNonces[key] =
			struct{}{}

		cloned := pending

		cloned.Proposal =
			cloneAuthorityProposal(
				pending.Proposal,
			)

		pendingProposals[proposalID] =
			cloned
	}

	return &GovernanceState{
		ChainID:          snapshot.ChainID,
		CurrentPolicy:    snapshot.CurrentPolicy,
		Replay:           replay,
		PendingProposals: pendingProposals,
	}, nil
}
