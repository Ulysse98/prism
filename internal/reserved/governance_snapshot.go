package reserved

import "fmt"

type GovernanceSnapshot struct {
	CurrentPolicy AuthorityPolicy `json:"current_policy"`
	Replay        ReplaySnapshot  `json:"replay"`
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

	return GovernanceSnapshot{
		CurrentPolicy: state.CurrentPolicy,
		Replay:        replaySnapshot,
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

	return &GovernanceState{
		CurrentPolicy: snapshot.CurrentPolicy,
		Replay:        replay,
	}, nil
}
