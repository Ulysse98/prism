package usefulwork

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

func TestQuantumAuditProvisionalTwoOfThree(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	audit, err := AuditQuantumVerification(proof, reports, policy)
	if err != nil {
		t.Fatal(err)
	}

	if !audit.QuorumSatisfied ||
		audit.AllReportsPresent ||
		audit.Status != QuantumAuditProvisional {
		t.Fatalf("unexpected provisional audit: %+v", audit)
	}

	if audit.PolicyFingerprint == "" {
		t.Fatal("missing policy fingerprint")
	}
}

func TestQuantumAuditAllReportsAccepted(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
		signedQuantumReport(t, proof, verifiers[2]),
	}

	audit, err := AuditQuantumVerification(proof, reports, policy)
	if err != nil {
		t.Fatal(err)
	}

	if !audit.QuorumSatisfied ||
		!audit.AllReportsPresent ||
		audit.Status != QuantumAuditComplete ||
		audit.Approvals != 3 {
		t.Fatalf("unexpected complete audit: %+v", audit)
	}
}

func TestQuantumAuditRejectsInvalidReport(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	first := signedQuantumReport(t, proof, verifiers[0])
	second := signedQuantumReport(t, proof, verifiers[1])

	second.Signature = "00"

	audit, err := AuditQuantumVerification(
		proof,
		[]QuantumVerificationReport{first, second},
		policy,
	)

	if err == nil ||
		audit.QuorumSatisfied ||
		audit.Status != QuantumAuditNotAccepted {
		t.Fatalf("invalid report accepted: %+v, err=%v", audit, err)
	}
}

func TestQuantumAuditSignedRejection(t *testing.T) {
	proof, worker, verifiers, policy := quantumQuorumFixture(t)

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

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	audit, err := AuditQuantumVerification(proof, reports, policy)

	if err == nil ||
		audit.QuorumSatisfied ||
		audit.Rejections != 2 ||
		audit.Status != QuantumAuditNotAccepted {
		t.Fatalf("signed rejection accepted: %+v, err=%v", audit, err)
	}
}

func TestQuantumAuditPolicyFingerprintStable(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	first, err := AuditQuantumVerification(proof, reports, policy)
	if err != nil {
		t.Fatal(err)
	}

	reordered := policy
	reordered.AuthorizedVerifiers = []string{
		policy.AuthorizedVerifiers[2],
		policy.AuthorizedVerifiers[0],
		policy.AuthorizedVerifiers[1],
	}

	second, err := AuditQuantumVerification(proof, reports, reordered)
	if err != nil {
		t.Fatal(err)
	}

	if first.PolicyFingerprint != second.PolicyFingerprint {
		t.Fatal("verifier ordering changed policy fingerprint")
	}

	changed := policy
	changed.RequiredApprovals = 3

	third, _ := AuditQuantumVerification(proof, reports, changed)

	if first.PolicyFingerprint == third.PolicyFingerprint {
		t.Fatal("different threshold produced identical fingerprint")
	}
}
