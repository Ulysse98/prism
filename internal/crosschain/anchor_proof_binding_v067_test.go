package crosschain

import (
	"strings"
	"testing"

	"prism/internal/compute"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

type anchorProofFixtureV067 struct {
	intent   AnchorIntentV067
	job      compute.Job
	contract usefulwork.ComputeTaskContractV066
	proof    usefulwork.Proof
	report   usefulwork.ComputeVerificationReportV064
	policy   AnchorProofPolicyV067
}

func newAnchorProofFixtureV067(t *testing.T) anchorProofFixtureV067 {
	t.Helper()
	task, err := usefulwork.NewMLInferenceQuantizedTask(
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
	job, err := compute.NewJob(task, "test-requester", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	const chain = "prism-da50ca9fa9c11da1"
	genesis := strings.Repeat("a", 64)
	artifact := strings.Repeat("c", 64)
	contract, err := usefulwork.NewQuantizedMLComputeTaskContractV066(task, job.ID, chain, genesis, artifact)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := usefulwork.ExecuteCompute(task, job.ID, chain, genesis, worker)
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Claim(worker.Address); err != nil {
		t.Fatal(err)
	}
	if err := job.Complete(proof); err != nil {
		t.Fatal(err)
	}
	report, err := usefulwork.SignComputeVerificationV064(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return anchorProofFixtureV067{
		intent: AnchorIntentV067{
			Version:             AnchorIntentVersionV067,
			SourceChainID:       chain,
			SourceGenesisHash:   genesis,
			SourceBlockHeight:   42, // Synthetic candidate metadata, NOT chain evidence.
			SourceBlockHash:     strings.Repeat("b", 64),
			JobID:               job.ID,
			ProofHash:           proof.ID,
			DestinationChainID:  "eip155:421614",
			DestinationRegistry: "0x1111111111111111111111111111111111111111",
			Nonce:               1,
		},
		job: job, contract: contract, proof: proof, report: report,
		policy: AnchorProofPolicyV067{
			SourceChainID:       chain,
			SourceGenesisHash:   genesis,
			ApprovedArtifactSHA: artifact,
			ApprovedVerifier:    verifier.Address,
			DestinationChainID:  "eip155:421614",
			DestinationRegistry: "0x1111111111111111111111111111111111111111",
		},
	}
}

func (f anchorProofFixtureV067) check() error {
	return VerifyAnchorCandidateProofV067(f.intent, f.job, f.contract, f.proof, f.report, f.policy)
}

func TestAnchorCandidateIndependentPoUWV067(t *testing.T) {
	f := newAnchorProofFixtureV067(t)
	if err := f.check(); err != nil {
		t.Fatalf("valid signed off-chain triple rejected: %v", err)
	}
	if _, err := f.intent.Digest(); err != nil {
		t.Fatal(err)
	}
}

func TestAnchorCandidateRejectsTamperingV067(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*anchorProofFixtureV067)
	}{
		{"wrong-source-chain", func(f *anchorProofFixtureV067) { f.intent.SourceChainID = "prism-other" }},
		{"wrong-source-genesis", func(f *anchorProofFixtureV067) { f.intent.SourceGenesisHash = strings.Repeat("e", 64) }},
		{"wrong-destination-chain", func(f *anchorProofFixtureV067) { f.intent.DestinationChainID = "eip155:10143" }},
		{"wrong-destination-registry", func(f *anchorProofFixtureV067) {
			f.intent.DestinationRegistry = "0x2222222222222222222222222222222222222222"
		}},
		{"missing-intent-nonce", func(f *anchorProofFixtureV067) { f.intent.Nonce = 0 }},
		{"wrong-intent-job", func(f *anchorProofFixtureV067) { f.intent.JobID = strings.Repeat("e", 64) }},
		{"wrong-intent-proof", func(f *anchorProofFixtureV067) { f.intent.ProofHash = strings.Repeat("e", 64) }},
		{"wrong-job-status", func(f *anchorProofFixtureV067) { f.job.Status = compute.JobStatusClaimed }},
		{"wrong-job-id", func(f *anchorProofFixtureV067) { f.job.ID = strings.Repeat("e", 64) }},
		{"wrong-job-proof", func(f *anchorProofFixtureV067) { f.job.ProofID = strings.Repeat("e", 64) }},
		{"wrong-job-worker", func(f *anchorProofFixtureV067) { f.job.Worker = "other-worker" }},
		{"wrong-job-score", func(f *anchorProofFixtureV067) { f.job.WorkUnits++ }},
		{"wrong-job-task", func(f *anchorProofFixtureV067) { f.job.Task.InputHash = strings.Repeat("e", 64) }},
		{"wrong-contract-job", func(f *anchorProofFixtureV067) { f.contract.JobID = "different-job" }},
		{"wrong-contract-artifact", func(f *anchorProofFixtureV067) { f.contract.ArtifactHash = strings.Repeat("e", 64) }},
		{"wrong-proof-result", func(f *anchorProofFixtureV067) {
			f.proof.ResultValues = append([]uint64(nil), f.proof.ResultValues...)
			f.proof.ResultValues[0] = (f.proof.ResultValues[0] + 1) % f.proof.Task.ColsB
		}},
		{"wrong-proof-signature", func(f *anchorProofFixtureV067) { f.proof.Signature = "00" }},
		{"wrong-report-signature", func(f *anchorProofFixtureV067) { f.report.Signature = "00" }},
		{"wrong-report-proof", func(f *anchorProofFixtureV067) { f.report.ProofID = strings.Repeat("e", 64) }},
		{"wrong-approved-verifier", func(f *anchorProofFixtureV067) { f.policy.ApprovedVerifier = "other-verifier" }},
		{"missing-approved-verifier", func(f *anchorProofFixtureV067) { f.policy.ApprovedVerifier = "" }},
		{"wrong-approved-artifact", func(f *anchorProofFixtureV067) { f.policy.ApprovedArtifactSHA = strings.Repeat("e", 64) }},
		{"wrong-policy-genesis", func(f *anchorProofFixtureV067) { f.policy.SourceGenesisHash = strings.Repeat("e", 64) }},
		{"wrong-policy-destination", func(f *anchorProofFixtureV067) {
			f.policy.DestinationRegistry = "0x2222222222222222222222222222222222222222"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAnchorProofFixtureV067(t)
			tc.mutate(&f)
			if err := f.check(); err == nil {
				t.Fatal("invalid off-chain candidate unexpectedly accepted")
			}
		})
	}
}

// Deliberate limitation: changing a synthetic block height/hash to ANOTHER
// valid value still passes, proving that this layer does not check inclusion.
func TestAnchorCandidateDoesNotProveBlockInclusionV067(t *testing.T) {
	f := newAnchorProofFixtureV067(t)
	f.intent.SourceBlockHeight = 777
	f.intent.SourceBlockHash = strings.Repeat("f", 64)
	if err := f.check(); err != nil {
		t.Fatalf("candidate-binding-only contract unexpectedly checks block inclusion: %v", err)
	}
}
