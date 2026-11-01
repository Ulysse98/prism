package crosschain

import (
	"fmt"
	"reflect"

	"prism/internal/compute"
	"prism/internal/usefulwork"
)

// AnchorProofPolicyV067 MUST be pinned by an independent trusted operator.
// Never populate it from the untrusted proof, report, request or intent.
// This policy alone does not establish source-block inclusion/finality.
type AnchorProofPolicyV067 struct {
	SourceChainID       string
	SourceGenesisHash   string
	ApprovedArtifactSHA string
	ApprovedVerifier    string
	DestinationChainID  string
	DestinationRegistry string
}

// VerifyAnchorCandidateProofV067 validates an OFF-CHAIN candidate's cryptographic
// PoUW V2 proof, independently signed V0.64 verifier report, V0.66 task contract,
// marketplace job, and destination-specific anchor intent against pinned policy.
// SourceBlockHeight and SourceBlockHash are only structurally validated by
// AnchorIntentV067: this function DOES NOT verify block inclusion, consensus,
// network finality, replay uniqueness, bridge authorization, or settlement.
// A nil error MUST NEVER be used as permission to register/payout anything.
func VerifyAnchorCandidateProofV067(
	intent AnchorIntentV067,
	job compute.Job,
	contract usefulwork.ComputeTaskContractV066,
	proof usefulwork.Proof,
	report usefulwork.ComputeVerificationReportV064,
	policy AnchorProofPolicyV067,
) error {
	if err := intent.Validate(); err != nil {
		return fmt.Errorf("v0.67 invalid candidate intent: %w", err)
	}
	if !chainIDV067.MatchString(policy.SourceChainID) ||
		!chainIDV067.MatchString(policy.DestinationChainID) ||
		!registryIDV067.MatchString(policy.DestinationRegistry) ||
		policy.ApprovedVerifier == "" {
		return fmt.Errorf("v0.67 missing or invalid trusted source/destination/verifier policy")
	}
	if err := validHashV067("policy genesis hash", policy.SourceGenesisHash); err != nil {
		return err
	}
	if err := validHashV067("approved artifact hash", policy.ApprovedArtifactSHA); err != nil {
		return err
	}
	if intent.SourceChainID != policy.SourceChainID ||
		intent.SourceGenesisHash != policy.SourceGenesisHash ||
		intent.DestinationChainID != policy.DestinationChainID ||
		intent.DestinationRegistry != policy.DestinationRegistry {
		return fmt.Errorf("v0.67 intent does not match pinned policy")
	}
	if job.Status != compute.JobStatusVerified ||
		job.ID != compute.CalculateJobID(job) ||
		job.ID != intent.JobID ||
		job.ProofID != intent.ProofHash ||
		job.Worker == "" || job.Worker != proof.Worker ||
		job.WorkUnits == 0 || job.WorkUnits != proof.Score ||
		!reflect.DeepEqual(job.Task, proof.Task) {
		return fmt.Errorf("v0.67 marketplace job does not match candidate proof")
	}
	if proof.JobID != intent.JobID ||
		proof.ID != intent.ProofHash ||
		proof.ChainID != policy.SourceChainID ||
		proof.GenesisHash != policy.SourceGenesisHash {
		return fmt.Errorf("v0.67 proof identity or source context mismatch")
	}
	if contract.JobID != intent.JobID ||
		contract.ChainID != policy.SourceChainID ||
		contract.GenesisHash != policy.SourceGenesisHash {
		return fmt.Errorf("v0.67 compute contract source context mismatch")
	}
	// Recomputes full ML result and verifies worker Ed25519 proof signature,
	// report signature/verifier identity, contract/task/artifact bindings.
	if err := usefulwork.VerifyIndependentQuantizedMLComputeV066(
		contract, proof, report,
		policy.ApprovedArtifactSHA, policy.ApprovedVerifier,
	); err != nil {
		return fmt.Errorf("v0.67 independent PoUW proof rejected: %w", err)
	}
	return nil
}
