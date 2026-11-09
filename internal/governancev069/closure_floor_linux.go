//go:build linux

package governancev069

import (
	"errors"
	"fmt"
)

const (
	AdvisoryFloorRecoveryVerified = "EXTERNAL_FLOOR_MATCH_LOCAL_ONLY"
	AdvisoryFloorRecoveryRejected = "EXTERNAL_FLOOR_VIOLATED_FAIL_CLOSED"
)

// AdvisoryFloorRecoveryInspection is read-only. It checks a signed local
// marker against an externally retained exact fingerprint. It cannot prevent
// a privileged operator from altering the store, or detect a rollback if the
// caller also replaces the independent floor.
type AdvisoryFloorRecoveryInspection struct {
	Status        string `json:"status"`
	ClosureSHA256 string `json:"closure_sha256,omitempty"`
	FloorSHA256   string `json:"floor_sha256,omitempty"`
}

func InspectLocalAdvisoryClosureAgainstFloor(store *AdvisoryVoteReplayStore, externalFloor AdvisoryClosureRecoveryFloor) (AdvisoryFloorRecoveryInspection, error) {
	rejected := AdvisoryFloorRecoveryInspection{Status: AdvisoryFloorRecoveryRejected}
	if store == nil {
		return rejected, errors.New("nil advisory store")
	}
	if err := ValidateAdvisoryClosureRecoveryFloor(externalFloor); err != nil {
		return rejected, err
	}
	gate, err := localAdvisoryGate(store.dir)
	if err != nil {
		return rejected, err
	}
	defer gate.Close()
	// An expected signed closure missing after restore is a rollback signal,
	// never an empty election that can be treated as recovered.
	exists, err := gateHasClosure(int(gate.Fd()))
	if err != nil {
		return rejected, err
	}
	if !exists {
		return rejected, errors.New("externally pinned closure marker missing: possible rollback")
	}
	actual, err := readLocalClosureFromFD(int(gate.Fd()))
	if err != nil {
		return rejected, fmt.Errorf("invalid local closure record: %w", err)
	}
	if err := VerifyAdvisoryClosureAgainstFloor(actual, externalFloor); err != nil {
		return rejected, err
	}
	return AdvisoryFloorRecoveryInspection{
		Status: AdvisoryFloorRecoveryVerified, ClosureSHA256: actual.ClosureSHA256, FloorSHA256: externalFloor.FloorSHA256,
	}, nil
}
