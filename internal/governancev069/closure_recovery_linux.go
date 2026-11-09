//go:build linux

package governancev069

import (
	"errors"
	"fmt"
)

const (
	AdvisoryClosureRecoveryAbsent       = "NO_LOCAL_MARKER_NOT_AUTHORIZATION"
	AdvisoryClosureRecoveryVerified     = "SIGNED_LOCAL_MARKER_VERIFIED_NOT_GLOBAL_FINALITY"
	AdvisoryClosureRecoveryUnverifiable = "MARKER_PRESENT_UNVERIFIABLE_FAIL_CLOSED"
)

// AdvisoryClosureRecoveryInspection is a read-only inspection of ONE local
// marker's signature under an independent operator public-key pin. It does not
// prove vote completeness: run VerifySealedLocalAdvisoryVoteStore with the
// complete independently pinned roster, ballot set and tally for that.
type AdvisoryClosureRecoveryInspection struct {
	Status             string `json:"status"`
	ClosureSHA256      string `json:"closure_sha256,omitempty"`
	SignerPublicKeyHex string `json:"signer_public_key_hex,omitempty"`
}

// InspectLocalAdvisoryClosureRecovery never removes, repairs or rewrites a
// closure marker. An existing malformed or untrusted marker is fail-closed.
func InspectLocalAdvisoryClosureRecovery(store *AdvisoryVoteReplayStore, pinnedPublicKeyHex string) (AdvisoryClosureRecoveryInspection, error) {
	bad := AdvisoryClosureRecoveryInspection{Status: AdvisoryClosureRecoveryUnverifiable}
	if store == nil {
		return bad, errors.New("nil store")
	}
	if !isLowerSHA256(pinnedPublicKeyHex) {
		return bad, errors.New("invalid independently pinned Ed25519 operator key")
	}
	gate, err := localAdvisoryGate(store.dir)
	if err != nil {
		return bad, err
	}
	defer gate.Close()
	exists, err := gateHasClosure(int(gate.Fd()))
	if err != nil {
		return bad, err
	}
	if !exists {
		return AdvisoryClosureRecoveryInspection{Status: AdvisoryClosureRecoveryAbsent}, nil
	}
	closure, err := readLocalClosureFromFD(int(gate.Fd()))
	if err != nil {
		return bad, fmt.Errorf("closure marker exists but fails local inspection: %w", err)
	}
	if err := VerifyAdvisoryClosureAuthorization(closure, closure.Authorization, pinnedPublicKeyHex); err != nil {
		return bad, fmt.Errorf("closure marker exists but operator is not independently trusted: %w", err)
	}
	return AdvisoryClosureRecoveryInspection{
		Status:             AdvisoryClosureRecoveryVerified,
		ClosureSHA256:      closure.ClosureSHA256,
		SignerPublicKeyHex: closure.Authorization.SignerPublicKeyHex,
	}, nil
}
