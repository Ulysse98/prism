package reserved

import "fmt"

type GovernanceState struct {
	ChainID       string
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
		return nil,
			fmt.Errorf(
				"invalid initial reserved authority policy: %w",
				err,
			)
	}

	return &GovernanceState{
		CurrentPolicy: initialPolicy,
		Replay:        NewReplayState(),
	}, nil
}

func NewGovernanceStateForChain(
	chainID string,
	initialPolicy AuthorityPolicy,
) (
	*GovernanceState,
	error,
) {
	if chainID == "" {
		return nil,
			fmt.Errorf(
				"reserved governance chain ID cannot be empty",
			)
	}

	state, err :=
		NewGovernanceState(
			initialPolicy,
		)

	if err != nil {
		return nil, err
	}

	state.ChainID = chainID

	return state, nil
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

	if state.ChainID != "" &&
		expectedChainID != state.ChainID {

		return fmt.Errorf(
			"reserved governance chain ID mismatch",
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
