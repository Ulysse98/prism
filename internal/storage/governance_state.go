package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/reserved"
)

const governanceStateFilename = "reserved-governance.json"

func SaveGovernanceState(
	dataDir string,
	state *reserved.GovernanceState,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved governance state cannot be nil",
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
			"cannot encode reserved governance state: %w",
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
		governanceStateFilename,
	)

	if err := os.WriteFile(
		path,
		data,
		0600,
	); err != nil {
		return fmt.Errorf(
			"cannot save reserved governance state: %w",
			err,
		)
	}

	return nil
}

func LoadGovernanceState(
	dataDir string,
) (
	*reserved.GovernanceState,
	error,
) {
	path := filepath.Join(
		dataDir,
		governanceStateFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil,
			fmt.Errorf(
				"cannot load reserved governance state: %w",
				err,
			)
	}

	var snapshot reserved.GovernanceSnapshot

	if err := json.Unmarshal(
		data,
		&snapshot,
	); err != nil {
		return nil,
			fmt.Errorf(
				"cannot decode reserved governance state: %w",
				err,
			)
	}

	state, err :=
		reserved.GovernanceStateFromSnapshot(
			snapshot,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"invalid reserved governance state: %w",
				err,
			)
	}

	return state, nil
}

func GovernanceStateExists(
	dataDir string,
) bool {
	path := filepath.Join(
		dataDir,
		governanceStateFilename,
	)

	_, err := os.Stat(path)

	return err == nil
}
