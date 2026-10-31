package usefulwork

import (
	"strings"
	"testing"

	"prism/internal/wallet"
)

func independentComputeFixtureV066(t *testing.T) (
	ComputeTaskContractV066, Proof, ComputeVerificationReportV064,
	string, *wallet.Wallet, *wallet.Wallet,
) {
	t.Helper()
	task, err := NewMLInferenceQuantizedTask(
		2, 3, 3,
		[]int64{1, 2, 3, 3, 2, 1},
		[]int64{1, 2, 0, 0, 1, 2, -1, 1, 1},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	artifact := strings.Repeat("c", 64)
	chain := "prism-v066-offline-test"
	genesis := strings.Repeat("a", 64)
	job := "v066-independent-job"
	contract, err := NewQuantizedMLComputeTaskContractV066(task, job, chain, genesis, artifact)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := ExecuteCompute(task, job, chain, genesis, worker)
	if err != nil {
		t.Fatal(err)
	}
	report, err := SignComputeVerificationV064(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return contract, proof, report, artifact, worker, verifier
}

func TestIndependentQuantizedMLComputeV066AcceptsValid(t *testing.T) {
	contract, proof, report, artifact, _, verifier := independentComputeFixtureV066(t)
	if err := VerifyIndependentQuantizedMLComputeV066(contract, proof, report, artifact, verifier.Address); err != nil {
		t.Fatalf("valid signed verifier report was rejected: %v", err)
	}
}

func TestIndependentQuantizedMLComputeV066RejectsMutations(t *testing.T) {
	contract, proof, report, artifact, worker, verifier := independentComputeFixtureV066(t)
	tests := []struct {
		name   string
		mutate func(*ComputeTaskContractV066, *Proof, *ComputeVerificationReportV064, *string, *string)
	}{
		{"wrong-approved-artifact", func(_ *ComputeTaskContractV066, _ *Proof, _ *ComputeVerificationReportV064, approved *string, _ *string) {
			*approved = strings.Repeat("d", 64)
		}},
		{"wrong-approved-verifier", func(_ *ComputeTaskContractV066, _ *Proof, _ *ComputeVerificationReportV064, _ *string, approved *string) {
			*approved = worker.Address
		}},
		{"missing-approved-verifier", func(_ *ComputeTaskContractV066, _ *Proof, _ *ComputeVerificationReportV064, _ *string, approved *string) {
			*approved = ""
		}},
		{"contract-job", func(c *ComputeTaskContractV066, _ *Proof, _ *ComputeVerificationReportV064, _ *string, _ *string) {
			c.JobID = "other-job"
		}},
		{"contract-input", func(c *ComputeTaskContractV066, _ *Proof, _ *ComputeVerificationReportV064, _ *string, _ *string) {
			c.InputHash = strings.Repeat("0", 64)
		}},
		{"contract-artifact", func(c *ComputeTaskContractV066, _ *Proof, _ *ComputeVerificationReportV064, _ *string, _ *string) {
			c.ArtifactHash = strings.Repeat("d", 64)
		}},
		{"proof-result", func(_ *ComputeTaskContractV066, p *Proof, _ *ComputeVerificationReportV064, _ *string, _ *string) {
			p.ResultValues = append([]uint64(nil), p.ResultValues...)
			p.ResultValues[0] = (p.ResultValues[0] + 1) % p.Task.ColsB
		}},
		{"proof-job", func(_ *ComputeTaskContractV066, p *Proof, _ *ComputeVerificationReportV064, _ *string, _ *string) {
			p.JobID = "other-job"
		}},
		{"report-signature", func(_ *ComputeTaskContractV066, _ *Proof, r *ComputeVerificationReportV064, _ *string, _ *string) {
			r.Signature = "00"
		}},
		{"report-proof-id", func(_ *ComputeTaskContractV066, _ *Proof, r *ComputeVerificationReportV064, _ *string, _ *string) {
			r.ProofID = strings.Repeat("e", 64)
		}},
		{"report-network", func(_ *ComputeTaskContractV066, _ *Proof, r *ComputeVerificationReportV064, _ *string, _ *string) {
			r.ChainID = "other-chain"
		}},
		{"report-verifier", func(_ *ComputeTaskContractV066, _ *Proof, r *ComputeVerificationReportV064, _ *string, _ *string) {
			r.VerifierAddress = worker.Address
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, p, r, approvedArtifact, approvedVerifier := contract, proof, report, artifact, verifier.Address
			tc.mutate(&c, &p, &r, &approvedArtifact, &approvedVerifier)
			if err := VerifyIndependentQuantizedMLComputeV066(c, p, r, approvedArtifact, approvedVerifier); err == nil {
				t.Fatal("tampered or unapproved triple was accepted")
			}
		})
	}
}

func TestIndependentQuantizedMLComputeV066RejectsCrossContextAndUntrustedVerifier(t *testing.T) {
	contract, proof, report, artifact, worker, verifier := independentComputeFixtureV066(t)
	otherProof, err := ExecuteCompute(proof.Task, "other-valid-job", proof.ChainID, proof.GenesisHash, worker)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyIndependentQuantizedMLComputeV066(contract, otherProof, report, artifact, verifier.Address); err == nil {
		t.Fatal("cross-job valid proof replay accepted")
	}
	if _, err := SignComputeVerificationV064(proof, worker); err == nil {
		t.Fatal("worker was allowed to self-verify")
	}
	otherVerifier, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	otherReport, err := SignComputeVerificationV064(proof, otherVerifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyIndependentQuantizedMLComputeV066(contract, proof, otherReport, artifact, verifier.Address); err == nil {
		t.Fatal("unapproved, otherwise valid verifier report accepted")
	}
}
