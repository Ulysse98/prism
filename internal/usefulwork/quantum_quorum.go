package usefulwork

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type QuantumQuorumPolicy struct {
	Model               string
	AuthorizedVerifiers []string
	RequiredApprovals   int
}

type QuantumQuorumResult struct {
	Accepted          bool
	Approvals         int
	Rejections        int
	ReportsChecked    int
	RequiredApprovals int
}

func validateQuantumVerifierAddress(address string) error {
	const prefix = "prism_"

	if !strings.HasPrefix(address, prefix) ||
		len(address) != len(prefix)+40 ||
		address != strings.ToLower(address) {
		return fmt.Errorf("invalid verifier address")
	}

	raw, err := hex.DecodeString(address[len(prefix):])
	if err != nil || len(raw) != 20 {
		return fmt.Errorf("invalid verifier address encoding")
	}

	return nil
}

func VerifyQuantumQuorum(
	proof Proof,
	reports []QuantumVerificationReport,
	policy QuantumQuorumPolicy,
) (QuantumQuorumResult, error) {

	result := QuantumQuorumResult{
		RequiredApprovals: policy.RequiredApprovals,
	}

	if proof.ProofVersion != ComputeProofVersion ||
		proof.Task.Type != TaskTypeQuantumSimulation {
		return result, fmt.Errorf("quorum requires quantum Proof v2")
	}

	if policy.Model != "bell-ideal-v1" {
		return result, fmt.Errorf("unsupported quorum model")
	}

	if len(policy.AuthorizedVerifiers) == 0 {
		return result, fmt.Errorf("empty verifier allowlist")
	}

	if policy.RequiredApprovals < 1 ||
		policy.RequiredApprovals > len(policy.AuthorizedVerifiers) {
		return result, fmt.Errorf("invalid quorum threshold")
	}

	allowed := make(
		map[string]struct{},
		len(policy.AuthorizedVerifiers),
	)

	for _, address := range policy.AuthorizedVerifiers {
		if err := validateQuantumVerifierAddress(address); err != nil {
			return result, err
		}

		if address == proof.Worker {
			return result, fmt.Errorf("worker present in verifier allowlist")
		}

		if _, duplicate := allowed[address]; duplicate {
			return result, fmt.Errorf("duplicate authorized verifier")
		}

		allowed[address] = struct{}{}
	}

	if len(reports) == 0 {
		return result, fmt.Errorf("no verification reports")
	}

	if len(reports) > len(allowed) {
		return result, fmt.Errorf("too many verification reports")
	}

	seen := make(map[string]struct{}, len(reports))

	for _, report := range reports {
		address := report.VerifierAddress

		if _, ok := allowed[address]; !ok {
			return result, fmt.Errorf("unauthorized quantum verifier")
		}

		if _, duplicate := seen[address]; duplicate {
			return result, fmt.Errorf("duplicate verifier vote")
		}

		if err := VerifyQuantumVerificationReport(report, proof); err != nil {
			return result, fmt.Errorf(
				"invalid verifier report: %w",
				err,
			)
		}

		if report.Model != policy.Model {
			return result, fmt.Errorf("verifier model mismatch")
		}

		seen[address] = struct{}{}
		result.ReportsChecked++

		switch report.Decision {
		case QuantumDecisionAccept:
			result.Approvals++
		case QuantumDecisionReject:
			result.Rejections++
		default:
			return result, fmt.Errorf("unknown verifier decision")
		}
	}

	// Fail-closed policy: any authenticated rejection vetoes acceptance.
	if result.Rejections != 0 {
		return result, fmt.Errorf("quorum contains a signed rejection")
	}

	if result.Approvals < policy.RequiredApprovals {
		return result, fmt.Errorf(
			"insufficient approvals: got %d, need %d",
			result.Approvals,
			policy.RequiredApprovals,
		)
	}

	result.Accepted = true
	return result, nil
}
