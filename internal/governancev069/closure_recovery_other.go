//go:build !linux

package governancev069

import "errors"

const (
	AdvisoryClosureRecoveryAbsent       = "NO_LOCAL_MARKER_NOT_AUTHORIZATION"
	AdvisoryClosureRecoveryVerified     = "SIGNED_LOCAL_MARKER_VERIFIED_NOT_GLOBAL_FINALITY"
	AdvisoryClosureRecoveryUnverifiable = "MARKER_PRESENT_UNVERIFIABLE_FAIL_CLOSED"
)

type AdvisoryClosureRecoveryInspection struct {
	Status             string `json:"status"`
	ClosureSHA256      string `json:"closure_sha256,omitempty"`
	SignerPublicKeyHex string `json:"signer_public_key_hex,omitempty"`
}

func InspectLocalAdvisoryClosureRecovery(*AdvisoryVoteReplayStore, string) (AdvisoryClosureRecoveryInspection, error) {
	return AdvisoryClosureRecoveryInspection{Status: AdvisoryClosureRecoveryUnverifiable}, errors.New("v0.69 closure recovery inspection requires Linux")
}
