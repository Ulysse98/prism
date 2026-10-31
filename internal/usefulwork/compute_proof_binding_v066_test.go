package usefulwork

import (
	"strings"
	"testing"

	"prism/internal/wallet"
)

func quantizedMLBindingFixtureV066(t *testing.T) (ComputeTaskContractV066, Proof, string) {
	t.Helper()
	task, err := NewMLInferenceQuantizedTask(
		2, 2, 2,
		[]int64{2, -3, -1, 4},
		[]int64{2, 1, -1, -2},
		[]int64{1, -1},
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	const jobID = "job-v066-1"
	const chainID = "prism-testnet"
	genesisHash := strings.Repeat("a", 64)
	artifactHash := strings.Repeat("c", 64)
	contract, err := NewQuantizedMLComputeTaskContractV066(
		task, jobID, chainID, genesisHash, artifactHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := ExecuteCompute(task, jobID, chainID, genesisHash, worker)
	if err != nil {
		t.Fatal(err)
	}
	return contract, proof, artifactHash
}

func TestQuantizedMLComputeTaskBindingV066Valid(t *testing.T) {
	contract, proof, artifact := quantizedMLBindingFixtureV066(t)
	if err := VerifyQuantizedMLComputeTaskBindingV066(contract, proof, artifact); err != nil {
		t.Fatalf("valid v0.66 task/proof rejected: %v", err)
	}
	id, err := ComputeTaskContractIDV066(contract)
	if err != nil || len(id) != 64 {
		t.Fatalf("invalid contract commitment: %q %v", id, err)
	}
}

func TestQuantizedMLComputeTaskBindingV066RejectMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ComputeTaskContractV066, *Proof, *string)
	}{
		{"contract-job", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.JobID = "other-job" }},
		{"contract-chain", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.ChainID = "other-chain" }},
		{"contract-genesis", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.GenesisHash = strings.Repeat("b", 64) }},
		{"contract-task", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.TaskID = strings.Repeat("d", 64) }},
		{"contract-input", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.InputHash = strings.Repeat("e", 64) }},
		{"contract-workload", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.WorkloadID = "unknown-v1" }},
		{"contract-artifact", func(c *ComputeTaskContractV066, _ *Proof, _ *string) { c.ArtifactHash = strings.Repeat("d", 64) }},
		{"trusted-artifact", func(_ *ComputeTaskContractV066, _ *Proof, a *string) { *a = strings.Repeat("f", 64) }},
		{"missing-trusted-artifact", func(_ *ComputeTaskContractV066, _ *Proof, a *string) { *a = "" }},
		{"proof-version", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.ProofVersion = 1 }},
		{"proof-job", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.JobID = "other-job" }},
		{"proof-chain", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.ChainID = "other-chain" }},
		{"proof-genesis", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.GenesisHash = strings.Repeat("d", 64) }},
		{"proof-task-id", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.Task.ID = strings.Repeat("d", 64) }},
		{"proof-input-hash", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.Task.InputHash = strings.Repeat("d", 64) }},
		{"proof-output", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.OutputHash = strings.Repeat("d", 64) }},
		{"proof-signature", func(_ *ComputeTaskContractV066, p *Proof, _ *string) { p.Signature = "00" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			contract, proof, artifact := quantizedMLBindingFixtureV066(t)
			tc.mutate(&contract, &proof, &artifact)
			if err := VerifyQuantizedMLComputeTaskBindingV066(contract, proof, artifact); err == nil {
				t.Fatal("mutated v0.66 binding accepted")
			}
		})
	}
}

func TestQuantizedMLComputeTaskContractV066RejectUnsupportedTask(t *testing.T) {
	task, err := NewSumSquaresTask([]uint64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewQuantizedMLComputeTaskContractV066(
		task, "job-1", "prism-testnet", strings.Repeat("a", 64), strings.Repeat("b", 64),
	); err == nil {
		t.Fatal("non-quantized task accepted by v0.66 ML bridge")
	}
}

func TestQuantizedMLComputeTaskContractV066RejectInvalidArtifact(t *testing.T) {
	contract, proof, _ := quantizedMLBindingFixtureV066(t)
	if _, err := NewQuantizedMLComputeTaskContractV066(
		proof.Task, contract.JobID, contract.ChainID, contract.GenesisHash, "UNVERIFIED",
	); err == nil {
		t.Fatal("invalid artifact digest accepted")
	}
}
