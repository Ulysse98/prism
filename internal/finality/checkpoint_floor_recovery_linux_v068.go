//go:build linux

package finality

// Advisory, read-only inspection of an experimentally durable checkpoint floor.
// A caller-supplied external witness MUST be independently authenticated and
// monotonically retained. This code neither authenticates that witness nor
// repairs/commits a pending state, and MUST NOT authorize production signing.
import (
	"errors"
	"fmt"
)

type CheckpointFloorRecoveryStatusV068 string

const (
	// Ready means local committed state and supplied external witness match.
	// It is NOT a BFT finality or signer authorization assertion.
	CheckpointFloorReadyV068 CheckpointFloorRecoveryStatusV068 = "ready"
	// Pending: the current external witness has not advanced to the proposed one.
	CheckpointFloorAwaitingExternalV068 CheckpointFloorRecoveryStatusV068 = "awaiting_external"
	// Pending: the supplied external witness equals the proposed next witness.
	// Operator must explicitly call CommitPreparedCheckpointFloorV068.
	CheckpointFloorCommitRequiredV068 CheckpointFloorRecoveryStatusV068 = "commit_required"
)

// InspectCheckpointFloorRecoveryV068 observes local current/pending state under
// the Linux flock boundary and compares it to an EXTERNALLY TRUSTED witness.
// It never mutates state, exposes pending as current, starts a witness update,
// or automatically commits. The result is an advisory reconciliation hint only.
// A malicious caller can supply a stale but self-consistent external witness.
func InspectCheckpointFloorRecoveryV068(
	dir string, external TrustedCheckpointFloorWitnessV068,
) (CheckpointFloorRecoveryStatusV068, error) {
	var status CheckpointFloorRecoveryStatusV068
	err := withFloorStoreLockV068(dir, func(path string) error {
		record, err := readFloorRecordV068(path)
		if err != nil {
			return fmt.Errorf("checkpoint floor recovery read: %w", err)
		}
		if record.Pending == nil {
			if external != record.Witness {
				return errors.New("external witness does not match committed checkpoint floor")
			}
			status = CheckpointFloorReadyV068
			return nil
		}
		if record.PendingWitness == nil {
			return errors.New("prepared checkpoint missing pending witness")
		}
		switch external {
		case record.Witness:
			status = CheckpointFloorAwaitingExternalV068
		case *record.PendingWitness:
			status = CheckpointFloorCommitRequiredV068
		default:
			return errors.New("external witness matches neither committed nor pending checkpoint floor")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}
