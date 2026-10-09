package usefulwork

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"prism/internal/wallet"
)

func quantumReportTestSetup(
	t *testing.T,
) (Proof, *wallet.Wallet, *wallet.Wallet) {
	t.Helper()
	t.Setenv("PRISM_QUANTUM_BACKEND", "")

	task, err := NewQuantumSimulationTask(2, 4096)
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

	proof, err := ExecuteCompute(
		task,
		"quantum-job-001",
		"prism-test-chain",
		"test-genesis",
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	return proof, worker, verifier
}

func TestQuantumVerificationReportValid(t *testing.T) {
	proof, _, verifier := quantumReportTestSetup(t)

	report, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	if report.Decision != QuantumDecisionAccept {
		t.Fatalf("unexpected decision: %s", report.Decision)
	}

	if err := VerifyQuantumVerificationReport(report, proof); err != nil {
		t.Fatalf("valid signed report rejected: %v", err)
	}
}

func TestQuantumVerificationReportRejectDecision(t *testing.T) {
	proof, worker, verifier := quantumReportTestSetup(t)

	// Statistically unacceptable under the new reference model,
	// but within the older Proof v2 fixed 8% tolerance.
	proof.ResultValues = []uint64{2293, 0, 0, 1803}

	outputHash, err := calculateValuesOutputHash(proof.ResultValues)
	if err != nil {
		t.Fatal(err)
	}

	proof.OutputHash = outputHash
	proof.ID = CalculateComputeProofID(proof)
	proof.Signature = hex.EncodeToString(
		ed25519.Sign(worker.PrivateKey, []byte(proof.ID)),
	)

	if err := VerifyProof(proof); err != nil {
		t.Fatalf("legacy proof should be valid: %v", err)
	}

	report, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	if report.Decision != QuantumDecisionReject {
		t.Fatalf("expected REJECT, got %s", report.Decision)
	}

	if err := VerifyQuantumVerificationReport(report, proof); err != nil {
		t.Fatalf("valid rejection report rejected: %v", err)
	}
}

func TestQuantumVerificationReportRejectsTampering(t *testing.T) {
	proof, _, verifier := quantumReportTestSetup(t)

	original, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		mutate func(*QuantumVerificationReport)
	}{
		{"JobID", func(r *QuantumVerificationReport) { r.JobID = "other-job" }},
		{"ProofID", func(r *QuantumVerificationReport) { r.ProofID = "other-proof" }},
		{"ChainID", func(r *QuantumVerificationReport) { r.ChainID = "other-chain" }},
		{"GenesisHash", func(r *QuantumVerificationReport) { r.GenesisHash = "other-genesis" }},
		{"TaskID", func(r *QuantumVerificationReport) { r.TaskID = "other-task" }},
		{"OutputHash", func(r *QuantumVerificationReport) { r.OutputHash = "other-output" }},
		{"Model", func(r *QuantumVerificationReport) { r.Model = "other-model" }},
		{"Decision", func(r *QuantumVerificationReport) { r.Decision = QuantumDecisionReject }},
		{"Version", func(r *QuantumVerificationReport) { r.Version = 2 }},
		{"Verifier", func(r *QuantumVerificationReport) { r.VerifierAddress = "fake" }},
		{"PublicKey", func(r *QuantumVerificationReport) { r.VerifierPublicKey = "bad" }},
		{"ID", func(r *QuantumVerificationReport) { r.ID = "bad" }},
		{"Signature", func(r *QuantumVerificationReport) { r.Signature = "00" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := original
			tc.mutate(&tampered)

			if err := VerifyQuantumVerificationReport(tampered, proof); err == nil {
				t.Fatal("tampered report accepted")
			}
		})
	}
}

func TestQuantumVerificationReportRejectsSelfVerification(t *testing.T) {
	proof, worker, _ := quantumReportTestSetup(t)

	if _, err := SignQuantumVerificationReport(proof, worker); err == nil {
		t.Fatal("worker was allowed to self-verify")
	}
}

func TestQuantumVerificationReportRejectsReplay(t *testing.T) {
	proof, worker, verifier := quantumReportTestSetup(t)

	report, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	otherProof, err := ExecuteCompute(
		proof.Task,
		"quantum-job-002",
		proof.ChainID,
		proof.GenesisHash,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyQuantumVerificationReport(report, otherProof); err == nil {
		t.Fatal("cross-job report replay accepted")
	}
}

func TestQuantumVerificationAcceptance(t *testing.T) {
	proof, _, verifier := quantumReportTestSetup(t)

	report, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyQuantumVerificationAcceptance(report, proof); err != nil {
		t.Fatalf("valid accepted proof rejected: %v", err)
	}
}

func TestQuantumVerificationAcceptanceRejectsSignedRejection(t *testing.T) {
	proof, worker, verifier := quantumReportTestSetup(t)

	proof.ResultValues = []uint64{2293, 0, 0, 1803}

	hash, err := calculateValuesOutputHash(proof.ResultValues)
	if err != nil {
		t.Fatal(err)
	}

	proof.OutputHash = hash
	proof.ID = CalculateComputeProofID(proof)
	proof.Signature = hex.EncodeToString(
		ed25519.Sign(worker.PrivateKey, []byte(proof.ID)),
	)

	report, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	if report.Decision != QuantumDecisionReject {
		t.Fatalf("expected REJECT, got %s", report.Decision)
	}

	// A signed REJECT is a valid report.
	if err := VerifyQuantumVerificationReport(report, proof); err != nil {
		t.Fatalf("valid rejection report rejected: %v", err)
	}

	// But it must never count as an accepted result.
	if err := VerifyQuantumVerificationAcceptance(report, proof); err == nil {
		t.Fatal("signed rejection counted as acceptance")
	}

	// Changing REJECT to ACCEPT must also fail.
	report.Decision = QuantumDecisionAccept
	if err := VerifyQuantumVerificationAcceptance(report, proof); err == nil {
		t.Fatal("falsified acceptance was accepted")
	}
}
