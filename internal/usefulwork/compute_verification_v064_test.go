package usefulwork

import (
	"testing"

	"prism/internal/wallet"
)

func computeVerificationFixtureV064(t *testing.T) (Proof, *wallet.Wallet, *wallet.Wallet) {
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
	proof, err := ExecuteCompute(task, "job-v064-001", "test-chain-001", "genesis-001", worker)
	if err != nil {
		t.Fatal(err)
	}
	return proof, worker, verifier
}

func TestComputeVerificationV064Valid(t *testing.T) {
	proof, _, verifier := computeVerificationFixtureV064(t)
	report, err := SignComputeVerificationV064(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyComputeVerificationV064(report, proof); err != nil {
		t.Fatalf("valid attestation rejected: %v", err)
	}
	if report.WorkUnits != proof.Score {
		t.Fatal("unexpected amount of independently verified work")
	}
}

func TestComputeVerificationV064RejectMutations(t *testing.T) {
	proof, _, verifier := computeVerificationFixtureV064(t)
	original, err := SignComputeVerificationV064(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*ComputeVerificationReportV064)
	}{
		{"version", func(r *ComputeVerificationReportV064) { r.Version++ }},
		{"job", func(r *ComputeVerificationReportV064) { r.JobID = "other-job" }},
		{"proof", func(r *ComputeVerificationReportV064) { r.ProofID = "other-proof" }},
		{"chain", func(r *ComputeVerificationReportV064) { r.ChainID = "other-chain" }},
		{"genesis", func(r *ComputeVerificationReportV064) { r.GenesisHash = "other-genesis" }},
		{"task", func(r *ComputeVerificationReportV064) { r.TaskID = "other-task" }},
		{"input", func(r *ComputeVerificationReportV064) { r.InputHash = "other-input" }},
		{"output", func(r *ComputeVerificationReportV064) { r.OutputHash = "other-output" }},
		{"score", func(r *ComputeVerificationReportV064) { r.WorkUnits++ }},
		{"address", func(r *ComputeVerificationReportV064) { r.VerifierAddress = "invalid-address" }},
		{"public_key", func(r *ComputeVerificationReportV064) { r.VerifierPublicKey = "bad" }},
		{"digest", func(r *ComputeVerificationReportV064) { r.ID = "wrong-digest" }},
		{"signature", func(r *ComputeVerificationReportV064) { r.Signature = "00" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := original
			tc.mutate(&r)
			if err := VerifyComputeVerificationV064(r, proof); err == nil {
				t.Fatal("modified report was accepted")
			}
		})
	}
}

func TestComputeVerificationV064RejectReplayAndSelfVerification(t *testing.T) {
	proof, worker, verifier := computeVerificationFixtureV064(t)
	if _, err := SignComputeVerificationV064(proof, worker); err == nil {
		t.Fatal("worker was allowed to self-verify")
	}
	report, err := SignComputeVerificationV064(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, job, chain, genesis string }{
		{"job", "job-v064-002", proof.ChainID, proof.GenesisHash},
		{"chain", proof.JobID, "other-chain", proof.GenesisHash},
		{"genesis", proof.JobID, proof.ChainID, "other-genesis"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			other, err := ExecuteCompute(proof.Task, tc.job, tc.chain, tc.genesis, worker)
			if err != nil {
				t.Fatal(err)
			}
			if err := VerifyComputeVerificationV064(report, other); err == nil {
				t.Fatal("cross-context attestation replay accepted")
			}
		})
	}
}

func TestComputeVerificationV064RejectIncorrectMLResult(t *testing.T) {
	proof, worker, verifier := computeVerificationFixtureV064(t)
	wrong := append([]uint64(nil), proof.ResultValues...)
	wrong[0] = (wrong[0] + 1) % proof.Task.ColsB
	badProof, err := BuildComputeMLProofV2FromResult(
		proof.Task, proof.JobID, proof.ChainID, proof.GenesisHash, worker, wrong,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProof(badProof); err == nil {
		t.Fatal("incorrect ML inference passed native verification")
	}
	if _, err := SignComputeVerificationV064(badProof, verifier); err == nil {
		t.Fatal("independent verifier signed an incorrect ML result")
	}
}

func TestComputeVerificationV064RejectKeyMismatch(t *testing.T) {
	proof, _, verifier := computeVerificationFixtureV064(t)
	other, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	verifier.PublicKey = other.PublicKey
	if _, err := SignComputeVerificationV064(proof, verifier); err == nil {
		t.Fatal("verifier keypair mismatch was accepted")
	}
}
