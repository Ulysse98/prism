package reserved

import "fmt"

// ValidateReservedTransferQueueReplay checks whether a governed reserved
// transfer proposal is still eligible to enter the pending queue with
// respect to already-executed reserved spending.
//
// Grants and governed transfers intentionally share the same per-pool
// spending nonce domain.
func (state *AccountingState) ValidateReservedTransferQueueReplay(
	proposal ReservedTransferProposal,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved accounting state cannot be nil",
		)
	}

	if proposal.ID == "" {
		return fmt.Errorf(
			"reserved transfer proposal ID cannot be empty",
		)
	}

	if proposal.Nonce == 0 {
		return fmt.Errorf(
			"reserved transfer proposal nonce must be greater than zero",
		)
	}

	replay := state.Replay

	if replay == nil {
		return nil
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
				"reserved transfer nonce is not increasing: nonce=%d last=%d",
				proposal.Nonce,
				last,
			)
		}
	}

	return nil
}
