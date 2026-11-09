//go:build !linux

package governancev069

import "errors"

const (
	AdvisoryFloorRecoveryVerified = "EXTERNAL_FLOOR_MATCH_LOCAL_ONLY"
	AdvisoryFloorRecoveryRejected = "EXTERNAL_FLOOR_VIOLATED_FAIL_CLOSED"
)

type AdvisoryFloorRecoveryInspection struct {
	Status        string `json:"status"`
	ClosureSHA256 string `json:"closure_sha256,omitempty"`
	FloorSHA256   string `json:"floor_sha256,omitempty"`
}

func InspectLocalAdvisoryClosureAgainstFloor(*AdvisoryVoteReplayStore, AdvisoryClosureRecoveryFloor) (AdvisoryFloorRecoveryInspection, error) {
	return AdvisoryFloorRecoveryInspection{Status: AdvisoryFloorRecoveryRejected}, errors.New("external-floor local inspection requires Linux")
}
