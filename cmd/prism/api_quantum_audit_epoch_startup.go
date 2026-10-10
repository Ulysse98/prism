package main

import (
	"fmt"

	"prism/internal/usefulwork"
)

// configureQuantumAuditEpoch is called before the HTTP API
// begins serving requests. No audit policy means no journal.
func configureQuantumAuditEpoch(
	dataDir string,
	policy *usefulwork.QuantumQuorumPolicy,
	epoch uint64,
) error {
	if policy == nil {
		if epoch != 0 {
			return fmt.Errorf(
				"quantum audit epoch requires a configured policy",
			)
		}
		return nil
	}

	if epoch == 0 {
		return fmt.Errorf(
			"configured quantum audit requires a positive epoch",
		)
	}

	fingerprint, err :=
		usefulwork.QuantumAuditPolicyFingerprint(*policy)

	if err != nil {
		return fmt.Errorf(
			"unable to fingerprint quantum audit policy: %w",
			err,
		)
	}

	if err := recordQuantumAuditPolicyEpoch(
		dataDir, epoch, fingerprint,
	); err != nil {
		return fmt.Errorf(
			"quantum audit policy epoch rejected: %w",
			err,
		)
	}

	return nil
}
