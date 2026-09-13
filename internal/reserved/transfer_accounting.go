package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

// AcceptReservedTransfer applies one fully authorized reserved transfer
// to reserved accounting.
//
// Reserved transfers intentionally share the existing per-pool Grant
// nonce/replay domain. A pool therefore has one monotonically increasing
// spending sequence across legacy grants and governed transfers.
func (state *AccountingState) AcceptReservedTransfer(
	proposal ReservedTransferProposal,
	authorityPolicy AuthorityPolicy,
	expectedChainID string,
	supplyPolicy consensus.SupplyPolicy,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved accounting state cannot be nil",
		)
	}

	if err := supplyPolicy.Validate(); err != nil {
		return fmt.Errorf(
			"invalid supply policy: %w",
			err,
		)
	}

	if err :=
		authorityPolicy.ValidateReservedTransferProposal(
			proposal,
			expectedChainID,
		); err != nil {

		return err
	}

	replay := state.Replay

	if replay == nil {
		replay = NewReplayState()
	}

	if replay.usedGrantIDs != nil {
		if _, exists :=
			replay.usedGrantIDs[proposal.ID]; exists {

			return fmt.Errorf(
				"reserved transfer proposal already executed",
			)
		}
	}

	key := grantReplayKey{
		Pool: proposal.Pool,
	}

	if replay.lastGrantNonce != nil {
		if last, exists :=
			replay.lastGrantNonce[key]; exists &&
			proposal.Nonce <= last {

			return fmt.Errorf(
				"reserved transfer proposal nonce is not increasing: nonce=%d last=%d",
				proposal.Nonce,
				last,
			)
		}
	}

	nextUsage, err :=
		addAuthorizationUsage(
			state.Usage,
			Authorization{
				Pool:   proposal.Pool,
				Amount: proposal.Amount,
			},
		)

	if err != nil {
		return err
	}

	if err :=
		nextUsage.Validate(
			supplyPolicy,
		); err != nil {

		return fmt.Errorf(
			"reserved budget rejected transfer: %w",
			err,
		)
	}

	if replay.usedGrantIDs == nil {
		replay.usedGrantIDs =
			make(
				map[string]struct{},
			)
	}

	if replay.lastGrantNonce == nil {
		replay.lastGrantNonce =
			make(
				map[grantReplayKey]uint64,
			)
	}

	replay.usedGrantIDs[proposal.ID] =
		struct{}{}

	replay.lastGrantNonce[key] =
		proposal.Nonce

	state.Usage = nextUsage
	state.Replay = replay

	return nil
}
