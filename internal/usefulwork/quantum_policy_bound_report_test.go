package usefulwork

import (
	"encoding/json"
	"reflect"
	"testing"

	"prism/internal/wallet"
)

func TestQuantumPolicyBoundReportValid(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	bound, err := SignQuantumPolicyBoundReport(
		proof, verifiers[0], policy, 7,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyQuantumPolicyBoundReport(
		bound, proof, policy, 7,
	); err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(bound)
	if err != nil {
		t.Fatal(err)
	}

	var decoded QuantumPolicyBoundReport

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(bound, decoded) {
		t.Fatal("JSON round trip changed signed envelope")
	}

	if err := VerifyQuantumPolicyBoundReport(
		decoded, proof, policy, 7,
	); err != nil {
		t.Fatal(err)
	}
}

func TestQuantumPolicyBoundReportRejectsReplay(t *testing.T) {
	proof, _, verifiers, policy := quantumQuorumFixture(t)

	bound, err := SignQuantumPolicyBoundReport(
		proof, verifiers[0], policy, 7,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("NewEpoch", func(t *testing.T) {
		if err := VerifyQuantumPolicyBoundReport(
			bound, proof, policy, 8,
		); err == nil {
			t.Fatal("old epoch report accepted")
		}
	})

	t.Run("ChangedThreshold", func(t *testing.T) {
		changed := policy
		changed.RequiredApprovals = 3

		if err := VerifyQuantumPolicyBoundReport(
			bound, proof, changed, 7,
		); err == nil {
			t.Fatal("report accepted under changed policy")
		}
	})

	t.Run("ChangedVerifierRoster", func(t *testing.T) {
		changed := policy
		changed.AuthorizedVerifiers = []string{
			verifiers[1].Address,
			verifiers[2].Address,
		}

		if err := VerifyQuantumPolicyBoundReport(
			bound, proof, changed, 7,
		); err == nil {
			t.Fatal("removed verifier was accepted")
		}
	})

	t.Run("ChangedEnvelopeEpoch", func(t *testing.T) {
		altered := bound
		altered.PolicyEpoch = 8

		if err := VerifyQuantumPolicyBoundReport(
			altered, proof, policy, 8,
		); err == nil {
			t.Fatal("altered epoch passed signature verification")
		}
	})

	t.Run("ChangedFingerprint", func(t *testing.T) {
		altered := bound
		altered.PolicyFingerprint = "false"

		if err := VerifyQuantumPolicyBoundReport(
			altered, proof, policy, 7,
		); err == nil {
			t.Fatal("altered fingerprint accepted")
		}
	})

	t.Run("AlteredDecision", func(t *testing.T) {
		altered := bound
		altered.Report.Decision = QuantumDecisionReject

		if err := VerifyQuantumPolicyBoundReport(
			altered, proof, policy, 7,
		); err == nil {
			t.Fatal("modified original report accepted")
		}
	})

	t.Run("BadOuterSignature", func(t *testing.T) {
		altered := bound
		altered.Signature = "00"

		if err := VerifyQuantumPolicyBoundReport(
			altered, proof, policy, 7,
		); err == nil {
			t.Fatal("invalid policy signature accepted")
		}
	})

	t.Run("MissingOuterSignature", func(t *testing.T) {
		altered := bound
		altered.Signature = ""

		if err := VerifyQuantumPolicyBoundReport(
			altered, proof, policy, 7,
		); err == nil {
			t.Fatal("unsigned envelope accepted")
		}
	})
}

func TestQuantumPolicyBoundReportRejectsInvalidSigner(t *testing.T) {
	proof, worker, verifiers, policy := quantumQuorumFixture(t)

	outsider, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		signer *wallet.Wallet
		epoch  uint64
		policy QuantumQuorumPolicy
	}{
		{"Unauthorized", outsider, 7, policy},
		{"Worker", worker, 7, policy},
		{"ZeroEpoch", verifiers[0], 0, policy},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SignQuantumPolicyBoundReport(
				proof, tc.signer, tc.policy, tc.epoch,
			); err == nil {
				t.Fatal("invalid policy signing was permitted")
			}
		})
	}
}
