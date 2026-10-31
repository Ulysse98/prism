package usefulwork

import "fmt"

// VerifyIndependentQuantizedMLComputeV066 validates an off-chain triple:
// independently approved compute contract, signed PoUW v2 proof, and an
// independent verifier's signed v0.64 CPU re-execution report.
//
// The approved artifact hash and verifier address MUST be chosen by an
// external trusted policy, never copied from the untrusted request/report.
// This is NOT a one-time replay registry, GPU hardware attestation, or a
// consensus/escrow authorization. Proof/report reuse for the SAME job is not
// prevented here; duplicate settlements must be guarded by consensus state.
func VerifyIndependentQuantizedMLComputeV066(
	contract ComputeTaskContractV066,
	proof Proof,
	report ComputeVerificationReportV064,
	approvedArtifactSHA256 string,
	approvedVerifierAddress string,
) error {
	if approvedVerifierAddress == "" || report.VerifierAddress != approvedVerifierAddress {
		return fmt.Errorf("v0.66 verifier is not approved by the external policy")
	}
	if err := VerifyQuantizedMLComputeTaskBindingV066(contract, proof, approvedArtifactSHA256); err != nil {
		return fmt.Errorf("v0.66 compute proof/contract rejected: %w", err)
	}
	if err := VerifyComputeVerificationV064(report, proof); err != nil {
		return fmt.Errorf("v0.66 independent verifier report rejected: %w", err)
	}
	return nil
}
