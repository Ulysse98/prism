package crosschain

import (
	"errors"
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/consensus"
	"prism/internal/usefulwork"
)

// LocalAnchorCandidateV067 holds a candidate's untrusted material.
// It has NO settlement authority; do not populate trusted policies from it.
type LocalAnchorCandidateV067 struct {
	Intent   AnchorIntentV067
	Job      compute.Job
	Contract usefulwork.ComputeTaskContractV066
	Proof    usefulwork.Proof
	Report   usefulwork.ComputeVerificationReportV064
}

// AdmitLocalAnchorCandidateV067 performs one fixed verification sequence before
// attempting a one-shot LOCAL reservation. It checks independent proof binding,
// locally validated chain inclusion, and a trusted operator-pinned checkpoint.
//
// TRUST REQUIREMENTS: The caller must supply an independently authenticated
// operator policy/checkpoint, immutable trusted chain snapshot/validator set,
// and a private trusted LocalReplayStoreV067. Never accept those values from an
// untrusted request or let the chain/validator set mutate during verification.
//
// IMPORTANT: This is an experimental, OFFLINE candidate-admission helper.
// A pinned local checkpoint does NOT prove network-wide canonicality or BFT
// finality. Success NEVER permits registry submission, transaction signing,
// funds transfer, escrow release, or settlement. An external caller able to
// access ReserveProofV067 directly can still bypass this helper; a future
// privileged admission boundary must prevent that before network exposure.
func AdmitLocalAnchorCandidateV067(
	store *LocalReplayStoreV067,
	candidate LocalAnchorCandidateV067,
	policy AnchorProofPolicyV067,
	checkpoint PinnedSourceCheckpointV067,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) (LocalReplayReservationV067, error) {
	return admitLocalAnchorCandidateWithPreflightV067(store, candidate.Intent, func() error {
		return VerifyAnchorWithPinnedCheckpointV067(
			candidate.Intent, candidate.Job, candidate.Contract,
			candidate.Proof, candidate.Report, policy, checkpoint, chain, pos,
		)
	})
}

// This unexported seam tests ordering/atomic reservation without forging a
// successful cryptographic verification fixture. Production uses only the
// fixed VerifyAnchorWithPinnedCheckpointV067 verifier above.
func admitLocalAnchorCandidateWithPreflightV067(
	store *LocalReplayStoreV067,
	intent AnchorIntentV067,
	preflight func() error,
) (LocalReplayReservationV067, error) {
	if store == nil || store.dir == "" || preflight == nil {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	if err := intent.Validate(); err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local admission invalid intent: %w", err)
	}
	if err := preflight(); err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local admission proof/checkpoint rejected: %w", err)
	}
	record, err := store.ReserveProofV067(intent)
	if err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local admission reservation rejected: %w", err)
	}
	return record, nil
}

// ErrLocalAnchorAdmissionReplayV067 permits the caller to identify a one-shot
// replay without granting any further permission. A reservation is not payout.
func ErrLocalAnchorAdmissionReplayV067(err error) bool {
	return errors.Is(err, ErrLocalReplayReservedV067)
}
