package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/reserved"
)

const replayStateFilename = "reserved-replay.json"

func SaveReplayState(
	dataDir string,
	state *reserved.ReplayState,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	snapshot, err := state.Snapshot()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(
		snapshot,
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf(
			"cannot encode reserved replay state: %w",
			err,
		)
	}

	if err := os.MkdirAll(
		dataDir,
		0755,
	); err != nil {
		return err
	}

	path := filepath.Join(
		dataDir,
		replayStateFilename,
	)

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		return fmt.Errorf(
			"cannot save reserved replay state: %w",
			err,
		)
	}

	return nil
}

func LoadReplayState(
	dataDir string,
) (
	*reserved.ReplayState,
	error,
) {
	path := filepath.Join(
		dataDir,
		replayStateFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot load reserved replay state: %w",
			err,
		)
	}

	var snapshot reserved.ReplaySnapshot

	if err := json.Unmarshal(
		data,
		&snapshot,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot decode reserved replay state: %w",
			err,
		)
	}

	state, err :=
		reserved.ReplayStateFromSnapshot(
			snapshot,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"invalid reserved replay state: %w",
			err,
		)
	}

	return state, nil
}

func ReplayStateExists(
	dataDir string,
) bool {
	path := filepath.Join(
		dataDir,
		replayStateFilename,
	)

	_, err := os.Stat(path)

	return err == nil
}
