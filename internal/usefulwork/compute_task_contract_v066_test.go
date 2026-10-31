package usefulwork

import (
	"strings"
	"testing"
)

func computeTaskContractFixtureV066() ComputeTaskContractV066 {
	return ComputeTaskContractV066{
		Version: ComputeTaskContractVersionV066,
		ChainID: "prism-testnet", GenesisHash: strings.Repeat("a", 64),
		JobID: "job-1", TaskID: "task-1", InputHash: strings.Repeat("b", 64),
		WorkloadID: "ml-inference-v1", ArtifactHash: strings.Repeat("c", 64),
	}
}

func TestComputeTaskContractV066Deterministic(t *testing.T) {
	c := computeTaskContractFixtureV066()
	id1, err := ComputeTaskContractIDV066(c)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := ComputeTaskContractIDV066(c)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 || len(id1) != 64 {
		t.Fatalf("non-deterministic digest: %q %q", id1, id2)
	}
}

func TestComputeTaskContractV066RejectInvalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ComputeTaskContractV066)
	}{
		{"version", func(c *ComputeTaskContractV066) { c.Version++ }},
		{"missing-chain", func(c *ComputeTaskContractV066) { c.ChainID = "" }},
		{"bad-job", func(c *ComputeTaskContractV066) { c.JobID = "job id" }},
		{"missing-task", func(c *ComputeTaskContractV066) { c.TaskID = "" }},
		{"bad-workload", func(c *ComputeTaskContractV066) { c.WorkloadID = "abc\n" }},
		{"genesis-length", func(c *ComputeTaskContractV066) { c.GenesisHash = "123" }},
		{"uppercase-input", func(c *ComputeTaskContractV066) { c.InputHash = strings.Repeat("A", 64) }},
		{"bad-artifact", func(c *ComputeTaskContractV066) { c.ArtifactHash = strings.Repeat("z", 64) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := computeTaskContractFixtureV066()
			tc.mutate(&c)
			if _, err := ComputeTaskContractIDV066(c); err == nil {
				t.Fatal("invalid contract accepted")
			}
		})
	}
}

func TestComputeTaskContractV066BindsFields(t *testing.T) {
	base := computeTaskContractFixtureV066()
	original, err := ComputeTaskContractIDV066(base)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*ComputeTaskContractV066)
	}{
		{"chain", func(c *ComputeTaskContractV066) { c.ChainID = "prism-other" }},
		{"genesis", func(c *ComputeTaskContractV066) { c.GenesisHash = strings.Repeat("d", 64) }},
		{"job", func(c *ComputeTaskContractV066) { c.JobID = "job-2" }},
		{"task", func(c *ComputeTaskContractV066) { c.TaskID = "task-2" }},
		{"input", func(c *ComputeTaskContractV066) { c.InputHash = strings.Repeat("e", 64) }},
		{"workload", func(c *ComputeTaskContractV066) { c.WorkloadID = "ml-inference-v2" }},
		{"artifact", func(c *ComputeTaskContractV066) { c.ArtifactHash = strings.Repeat("f", 64) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			changed, err := ComputeTaskContractIDV066(c)
			if err != nil {
				t.Fatal(err)
			}
			if changed == original {
				t.Fatal("mutated field did not affect commitment")
			}
		})
	}
}
