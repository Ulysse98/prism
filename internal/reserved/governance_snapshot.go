package reserved

import (
	"fmt"
	"sort"
)

type GovernanceSnapshot struct {
	ChainID       string          `json:"chain_id,omitempty"`
	CurrentPolicy AuthorityPolicy `json:"current_policy"`
	Replay        ReplaySnapshot  `json:"replay"`

	PendingProposals []PendingAuthorityProposal        `json:"pending_proposals,omitempty"`
	PendingTransfers []PendingReservedTransferProposal `json:"pending_transfers,omitempty"`
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

	pendingTransfers :=
		make(
			[]PendingReservedTransferProposal,
			0,
			len(state.PendingTransfers),
		)

	for proposalID, pending := range state.PendingTransfers {

		if proposalID !=
			pending.Proposal.ID {

			return GovernanceSnapshot{},
				fmt.Errorf(
					"pending reserved transfer map key mismatch",
				)
		}

		if err :=
			ValidatePendingReservedTransferProposal(
				pending,
			); err != nil {

			return GovernanceSnapshot{},
				fmt.Errorf(
					"invalid pending reserved transfer proposal: %w",
					err,
				)
		}

		cloned := pending

		cloned.Proposal =
			cloneReservedTransferProposal(
				pending.Proposal,
			)

		pendingTransfers =
			append(
				pendingTransfers,
				cloned,
			)
	}

	sort.Slice(
		pendingTransfers,
		func(
			i int,
			j int,
		) bool {
			return pendingTransfers[i].Proposal.ID <
				pendingTransfers[j].Proposal.ID
		},
	)

	return GovernanceSnapshot{
		ChainID:          state.ChainID,
		CurrentPolicy:    state.CurrentPolicy,
		Replay:           replaySnapshot,
		PendingProposals: pendingProposals,
		PendingTransfers: pendingTransfers,
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

	usedPendingAuthorityNonces :=
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
			usedPendingAuthorityNonces[key]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored pending authority proposal nonce",
				)
		}

		usedPendingAuthorityNonces[key] =
			struct{}{}

		cloned := pending

		cloned.Proposal =
			cloneAuthorityProposal(
				pending.Proposal,
			)

		pendingProposals[proposalID] =
			cloned
	}

	pendingTransfers :=
		make(
			map[string]PendingReservedTransferProposal,
			len(snapshot.PendingTransfers),
		)

	usedPendingTransferNonces :=
		make(
			map[poolNonce]struct{},
		)

	for _, pending := range snapshot.PendingTransfers {

		if err :=
			ValidatePendingReservedTransferProposal(
				pending,
			); err != nil {

			return nil,
				fmt.Errorf(
					"invalid stored pending reserved transfer proposal: %w",
					err,
				)
		}

		if snapshot.ChainID != "" &&
			pending.Proposal.ChainID !=
				snapshot.ChainID {

			return nil,
				fmt.Errorf(
					"stored pending reserved transfer proposal chain ID mismatch",
				)
		}

		proposalID :=
			pending.Proposal.ID

		if _, exists :=
			pendingTransfers[proposalID]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored pending reserved transfer proposal",
				)
		}

		key := poolNonce{
			Pool: string(
				pending.Proposal.Pool,
			),
			Nonce: pending.Proposal.Nonce,
		}

		if _, exists :=
			usedPendingTransferNonces[key]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored pending reserved transfer proposal nonce",
				)
		}

		usedPendingTransferNonces[key] =
			struct{}{}

		cloned := pending

		cloned.Proposal =
			cloneReservedTransferProposal(
				pending.Proposal,
			)

		pendingTransfers[proposalID] =
			cloned
	}

	return &GovernanceState{
		ChainID:          snapshot.ChainID,
		CurrentPolicy:    snapshot.CurrentPolicy,
		Replay:           replay,
		PendingProposals: pendingProposals,
		PendingTransfers: pendingTransfers,
	}, nil
}
