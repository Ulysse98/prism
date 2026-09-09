package reserved

import "fmt"

func (state *AccountingState) AcceptRevocation(
	revocation Revocation,
	authorityPolicy AuthorityPolicy,
	expectedChainID string,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved accounting state cannot be nil",
		)
	}

	replay := state.Replay

	if replay == nil {
		replay =
			NewReplayState()
	}

	if err :=
		replay.AcceptRevocation(
			revocation,
			authorityPolicy,
			expectedChainID,
		); err != nil {

		return err
	}

	state.Replay = replay

	return nil
}
