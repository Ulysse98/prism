package usefulwork

import "fmt"

// ComputeWorkloadQuantizedMLV066 is the sole workload supported by the initial
// off-chain v0.66 bridge. Expanding it requires an explicit semantics review.
const ComputeWorkloadQuantizedMLV066 = "ml-inference-quantized-v1"

// NewQuantizedMLComputeTaskContractV066 constructs a request commitment for a
// validated, existing quantized ML Task. artifactHash is a declared digest:
// callers must independently hash and approve the intended artifact bytes.
// This contract is unsigned and carries no settlement authority.
func NewQuantizedMLComputeTaskContractV066(
	task Task,
	jobID, chainID, genesisHash, artifactHash string,
) (ComputeTaskContractV066, error) {
	if task.Type != TaskTypeMLInferenceQuantized {
		return ComputeTaskContractV066{}, fmt.Errorf("v0.66 bridge only supports quantized ML tasks")
	}
	if err := ValidateTask(task); err != nil {
		return ComputeTaskContractV066{}, fmt.Errorf("invalid source task: %w", err)
	}

	contract := ComputeTaskContractV066{
		Version:      ComputeTaskContractVersionV066,
		ChainID:      chainID,
		GenesisHash:  genesisHash,
		JobID:        jobID,
		TaskID:       task.ID,
		InputHash:    task.InputHash,
		WorkloadID:   ComputeWorkloadQuantizedMLV066,
		ArtifactHash: artifactHash,
	}
	if err := ValidateComputeTaskContractV066(contract); err != nil {
		return ComputeTaskContractV066{}, err
	}
	return contract, nil
}

// VerifyQuantizedMLComputeTaskBindingV066 confirms that a complete, valid,
// signed context-bound PoUW v2 proof matches an independently supplied v0.66
// task contract. It recomputes the quantized ML result via VerifyProof.
//
// trustedArtifactHash MUST originate from independently validated input
// material or a trusted artifact policy, not from an untrusted proof or
// untrusted request. Equality alone does NOT prove which executable ran.
// This function does not authorize settlement or change consensus rules.
func VerifyQuantizedMLComputeTaskBindingV066(
	contract ComputeTaskContractV066,
	proof Proof,
	trustedArtifactHash string,
) error {
	if err := ValidateComputeTaskContractV066(contract); err != nil {
		return fmt.Errorf("invalid v0.66 contract: %w", err)
	}
	if !validSHA256HexV066(trustedArtifactHash) {
		return fmt.Errorf("invalid trusted artifact SHA-256 digest")
	}
	if contract.ArtifactHash != trustedArtifactHash {
		return fmt.Errorf("v0.66 artifact does not match trusted digest")
	}
	if contract.WorkloadID != ComputeWorkloadQuantizedMLV066 ||
		proof.Task.Type != TaskTypeMLInferenceQuantized {
		return fmt.Errorf("unsupported v0.66 workload or proof task type")
	}
	if err := VerifyComputeProofContext(
		proof, contract.JobID, contract.ChainID, contract.GenesisHash,
	); err != nil {
		return fmt.Errorf("v0.66 proof context mismatch: %w", err)
	}
	if contract.TaskID != proof.Task.ID ||
		contract.InputHash != proof.Task.InputHash {
		return fmt.Errorf("v0.66 proof task or input commitment mismatch")
	}
	if err := VerifyProof(proof); err != nil {
		return fmt.Errorf("invalid v0.66 bound PoUW proof: %w", err)
	}
	return nil
}
