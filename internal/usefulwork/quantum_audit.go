package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

const QuantumAuditVersion = 1

const (
	QuantumAuditProvisional = "QUORUM_PROVISIONAL"
	QuantumAuditComplete    = "ALL_REPORTS_ACCEPTED"
	QuantumAuditNotAccepted = "NOT_ACCEPTED"
)

type QuantumAuditResult struct {
	Version           uint8  `json:"version"`
	Mode              string `json:"mode"`
	JobID             string `json:"job_id"`
	ProofID           string `json:"proof_id"`
	ChainID           string `json:"chain_id"`
	PolicyFingerprint string `json:"policy_fingerprint"`
	Status            string `json:"status"`
	QuorumSatisfied   bool   `json:"quorum_satisfied"`
	AllReportsPresent bool   `json:"all_reports_present"`
	Approvals         int    `json:"approvals"`
	Rejections        int    `json:"rejections"`
	ReportsChecked    int    `json:"reports_checked"`
	RequiredApprovals int    `json:"required_approvals"`
	Reason            string `json:"reason,omitempty"`
}

func quantumAuditPolicyFingerprint(
	policy QuantumQuorumPolicy,
) (string, error) {

	addresses := append(
		[]string(nil),
		policy.AuthorizedVerifiers...,
	)
	sort.Strings(addresses)

	payload := struct {
		Model               string   `json:"model"`
		RequiredApprovals   int      `json:"required_approvals"`
		AuthorizedVerifiers []string `json:"authorized_verifiers"`
	}{
		Model:               policy.Model,
		RequiredApprovals:   policy.RequiredApprovals,
		AuthorizedVerifiers: addresses,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	preimage := append(
		[]byte("Prism/QuantumAuditPolicy/v1\x00"),
		data...,
	)

	hash := sha256.Sum256(preimage)
	return hex.EncodeToString(hash[:]), nil
}

// AuditQuantumVerification is observational only.
// It does not alter marketplace, blockchain or settlement state.
func AuditQuantumVerification(
	proof Proof,
	reports []QuantumVerificationReport,
	policy QuantumQuorumPolicy,
) (QuantumAuditResult, error) {

	result := QuantumAuditResult{
		Version:           QuantumAuditVersion,
		Mode:              "audit-only",
		JobID:             proof.JobID,
		ProofID:           proof.ID,
		ChainID:           proof.ChainID,
		Status:            QuantumAuditNotAccepted,
		RequiredApprovals: policy.RequiredApprovals,
	}

	fingerprint, err := quantumAuditPolicyFingerprint(policy)
	if err != nil {
		return result, err
	}

	result.PolicyFingerprint = fingerprint

	quorum, err := VerifyQuantumQuorum(
		proof,
		reports,
		policy,
	)

	result.Approvals = quorum.Approvals
	result.Rejections = quorum.Rejections
	result.ReportsChecked = quorum.ReportsChecked

	if err != nil {
		result.Reason = err.Error()
		return result, err
	}

	result.QuorumSatisfied = quorum.Accepted

	result.AllReportsPresent =
		quorum.ReportsChecked == len(policy.AuthorizedVerifiers)

	if result.AllReportsPresent {
		result.Status = QuantumAuditComplete
	} else {
		result.Status = QuantumAuditProvisional
	}

	return result, nil
}
