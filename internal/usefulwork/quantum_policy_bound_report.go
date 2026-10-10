package usefulwork

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"prism/internal/wallet"
)

const (
	QuantumPolicyBoundReportVersion = 2
	quantumPolicyBoundDomain        = "Prism/QuantumPolicyBoundReport/v2\x00"
)

// QuantumPolicyBoundReport preserves the original signed v1
// report while independently signing the trusted policy context.
type QuantumPolicyBoundReport struct {
	Version           uint8                     `json:"version"`
	Report            QuantumVerificationReport `json:"report"`
	PolicyEpoch       uint64                    `json:"policy_epoch"`
	PolicyFingerprint string                    `json:"policy_fingerprint"`
	Signature         string                    `json:"signature"`
}

func quantumPolicyBoundDigest(
	bound QuantumPolicyBoundReport,
) ([32]byte, error) {
	payload := struct {
		Version           uint8  `json:"version"`
		ReportID          string `json:"report_id"`
		VerifierAddress   string `json:"verifier_address"`
		PolicyEpoch       uint64 `json:"policy_epoch"`
		PolicyFingerprint string `json:"policy_fingerprint"`
	}{
		Version:           bound.Version,
		ReportID:          bound.Report.ID,
		VerifierAddress:   bound.Report.VerifierAddress,
		PolicyEpoch:       bound.PolicyEpoch,
		PolicyFingerprint: bound.PolicyFingerprint,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return [32]byte{}, err
	}

	preimage := append([]byte(quantumPolicyBoundDomain), data...)
	return sha256.Sum256(preimage), nil
}

func validateQuantumPolicyBoundSigner(
	proof Proof,
	address string,
	policy QuantumQuorumPolicy,
) error {
	if policy.Model != "bell-ideal-v1" {
		return fmt.Errorf("unsupported quantum policy model")
	}

	count := len(policy.AuthorizedVerifiers)

	if count == 0 || count > 64 ||
		policy.RequiredApprovals < 1 ||
		policy.RequiredApprovals > count {
		return fmt.Errorf("invalid quantum quorum threshold")
	}

	allowed := make(map[string]struct{}, count)

	for _, candidate := range policy.AuthorizedVerifiers {
		if err := validateQuantumVerifierAddress(candidate); err != nil {
			return err
		}

		if candidate == proof.Worker {
			return fmt.Errorf("worker in verifier allowlist")
		}

		if _, duplicate := allowed[candidate]; duplicate {
			return fmt.Errorf("duplicate authorized verifier")
		}

		allowed[candidate] = struct{}{}
	}

	if _, authorized := allowed[address]; !authorized {
		return fmt.Errorf("unauthorized quantum policy signer")
	}

	return nil
}

func SignQuantumPolicyBoundReport(
	proof Proof,
	verifier *wallet.Wallet,
	policy QuantumQuorumPolicy,
	epoch uint64,
) (QuantumPolicyBoundReport, error) {
	if epoch == 0 {
		return QuantumPolicyBoundReport{},
			fmt.Errorf("policy epoch must be positive")
	}

	report, err := SignQuantumVerificationReport(
		proof, verifier,
	)
	if err != nil {
		return QuantumPolicyBoundReport{}, err
	}

	if err := validateQuantumPolicyBoundSigner(
		proof, report.VerifierAddress, policy,
	); err != nil {
		return QuantumPolicyBoundReport{}, err
	}

	fingerprint, err := quantumAuditPolicyFingerprint(policy)
	if err != nil {
		return QuantumPolicyBoundReport{}, err
	}

	bound := QuantumPolicyBoundReport{
		Version:           QuantumPolicyBoundReportVersion,
		Report:            report,
		PolicyEpoch:       epoch,
		PolicyFingerprint: fingerprint,
	}

	digest, err := quantumPolicyBoundDigest(bound)
	if err != nil {
		return QuantumPolicyBoundReport{}, err
	}

	bound.Signature = hex.EncodeToString(
		ed25519.Sign(verifier.PrivateKey, digest[:]),
	)

	return bound, nil
}

func VerifyQuantumPolicyBoundReport(
	bound QuantumPolicyBoundReport,
	proof Proof,
	policy QuantumQuorumPolicy,
	expectedEpoch uint64,
) error {
	if bound.Version != QuantumPolicyBoundReportVersion {
		return fmt.Errorf("unsupported policy-bound report version")
	}

	if expectedEpoch == 0 || bound.PolicyEpoch != expectedEpoch {
		return fmt.Errorf("quantum policy epoch mismatch")
	}

	if err := VerifyQuantumVerificationReport(
		bound.Report, proof,
	); err != nil {
		return fmt.Errorf("invalid original report: %w", err)
	}

	if err := validateQuantumPolicyBoundSigner(
		proof, bound.Report.VerifierAddress, policy,
	); err != nil {
		return err
	}

	fingerprint, err := quantumAuditPolicyFingerprint(policy)
	if err != nil {
		return err
	}

	if bound.PolicyFingerprint != fingerprint {
		return fmt.Errorf("quantum policy fingerprint mismatch")
	}

	publicKey, err := wallet.DecodePublicKey(
		bound.Report.VerifierPublicKey,
	)
	if err != nil {
		return err
	}

	signature, err := hex.DecodeString(bound.Signature)
	if err != nil {
		return fmt.Errorf("invalid bound signature encoding: %w", err)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid bound signature size")
	}

	digest, err := quantumPolicyBoundDigest(bound)
	if err != nil {
		return err
	}

	if !ed25519.Verify(publicKey, digest[:], signature) {
		return fmt.Errorf("invalid quantum policy-bound signature")
	}

	return nil
}
