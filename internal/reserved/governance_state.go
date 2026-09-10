package reserved

import "fmt"

type GovernanceState struct {
	CurrentPolicy AuthorityPolicy
	Replay        *ReplayState
}

func NewGovernanceState(
	initialPolicy AuthorityPolicy,
) (
	*GovernanceState,
	error,
) {
	if err := initialPolicy.Validate(); err != nil {
		return nil, fmt.Errorf(
			"invalid initial reserved authority policy: %w",
			err,
		)
	}

	return &GovernanceState{
		CurrentPolicy: initialPolicy,
		Replay:        NewReplayState(),
	}, nil
}

func (state *GovernanceState) ApplyAuthorityChange(
	change AuthorityChange,
	expectedChainID string,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved governance state cannot be nil",
		)
	}

	if state.Replay == nil {
		state.Replay = NewReplayState()
	}

	updated, err :=
		state.Replay.AcceptAuthorityChange(
			change,
			state.CurrentPolicy,
			expectedChainID,
		)

	if err != nil {
		return err
	}

	state.CurrentPolicy = updated

	return nil
}
