package usefulwork

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"prism/internal/wallet"
)

func quantumQuorumFixture(
	t *testing.T,
) (Proof, *wallet.Wallet, []*wallet.Wallet, QuantumQuorumPolicy) {
	t.Helper()

	proof, worker, first := quantumReportTestSetup(t)

	second, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	third, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	verifiers := []*wallet.Wallet{first, second, third}

	policy := QuantumQuorumPolicy{
		Model: "bell-ideal-v1",
		AuthorizedVerifiers: []string{
			first.Address,
			second.Address,
			third.Address,
		},
		RequiredApprovals: 2,
	}

	return proof, worker, verifiers, policy
}

func signedQuantumReport(
	t *testing.T,
	proof Proof,
	verifier *wallet.Wallet,
) QuantumVerificationReport {
	t.Helper()

	report, err := SignQuantumVerificationReport(proof, verifier)
	if err != nil {
		t.Fatal(err)
	}

	return report
}

func TestQuantumQuorumTwoOfThree(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	result, err := VerifyQuantumQuorum(proof, reports, policy)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Accepted ||
		result.Approvals != 2 ||
		result.ReportsChecked != 2 {
		t.Fatalf("unexpected quorum result: %+v", result)
	}
}

func TestQuantumQuorumRejectsInsufficientApprovals(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
	}

	if _, err := VerifyQuantumQuorum(proof, reports, policy); err == nil {
		t.Fatal("single approval satisfied 2-of-3 quorum")
	}
}

func TestQuantumQuorumRejectsDuplicateVote(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	first := signedQuantumReport(t, proof, verifiers[0])
	reports := []QuantumVerificationReport{first, first}

	if _, err := VerifyQuantumQuorum(proof, reports, policy); err == nil {
		t.Fatal("duplicate verifier vote accepted")
	}
}

func TestQuantumQuorumRejectsUnauthorizedVerifier(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	stranger, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, stranger),
	}

	if _, err := VerifyQuantumQuorum(proof, reports, policy); err == nil {
		t.Fatal("unauthorized verifier accepted")
	}
}

func TestQuantumQuorumRejectsWorkerInAllowlist(t *testing.T) {
	proof, worker, verifiers, policy := quantumQuorumFixture(t)

	policy.AuthorizedVerifiers[2] = worker.Address

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	if _, err := VerifyQuantumQuorum(proof, reports, policy); err == nil {
		t.Fatal("worker included in trusted verifier roster")
	}
}

func TestQuantumQuorumRejectsTamperedVote(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	first := signedQuantumReport(t, proof, verifiers[0])
	second := signedQuantumReport(t, proof, verifiers[1])
	second.Decision = QuantumDecisionReject

	reports := []QuantumVerificationReport{first, second}

	if _, err := VerifyQuantumQuorum(proof, reports, policy); err == nil {
		t.Fatal("tampered verifier vote accepted")
	}
}

func TestQuantumQuorumRejectsSignedRejection(t *testing.T) {
	proof, worker, verifiers, policy := quantumQuorumFixture(t)

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

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	if reports[0].Decision != QuantumDecisionReject {
		t.Fatal("expected valid signed rejection")
	}

	result, err := VerifyQuantumQuorum(proof, reports, policy)
	if err == nil || result.Accepted {
		t.Fatal("signed rejection allowed quorum acceptance")
	}
}

func TestQuantumQuorumRejectsInvalidPolicy(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	reports := []QuantumVerificationReport{
		signedQuantumReport(t, proof, verifiers[0]),
		signedQuantumReport(t, proof, verifiers[1]),
	}

	t.Run("DuplicateRoster", func(t *testing.T) {
		bad := policy
		bad.AuthorizedVerifiers = []string{
			verifiers[0].Address,
			verifiers[0].Address,
			verifiers[2].Address,
		}

		if _, err := VerifyQuantumQuorum(proof, reports, bad); err == nil {
			t.Fatal("duplicate roster accepted")
		}
	})

	t.Run("ImpossibleThreshold", func(t *testing.T) {
		bad := policy
		bad.RequiredApprovals = 4

		if _, err := VerifyQuantumQuorum(proof, reports, bad); err == nil {
			t.Fatal("impossible threshold accepted")
		}
	})

	t.Run("WrongModel", func(t *testing.T) {
		bad := policy
		bad.Model = "unknown-model"

		if _, err := VerifyQuantumQuorum(proof, reports, bad); err == nil {
			t.Fatal("unknown model accepted")
		}
	})

	t.Run("InvalidAddress", func(t *testing.T) {
		bad := policy
		bad.AuthorizedVerifiers = []string{
			verifiers[0].Address,
			verifiers[1].Address,
			"invalid-address",
		}

		if _, err := VerifyQuantumQuorum(proof, reports, bad); err == nil {
			t.Fatal("invalid verifier address accepted")
		}
	})
}
