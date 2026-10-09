package usefulwork

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"prism/internal/wallet"
)

const (
	QuantumReportVersion  = 1
	QuantumDecisionAccept = "ACCEPT"
	QuantumDecisionReject = "REJECT"
	quantumReportDomain   = "Prism/QuantumVerificationReport/v1\x00"
)

type QuantumVerificationReport struct {
	Version           uint8  `json:"version"`
	JobID             string `json:"job_id"`
	ProofID           string `json:"proof_id"`
	ChainID           string `json:"chain_id"`
	GenesisHash       string `json:"genesis_hash"`
	TaskID            string `json:"task_id"`
	OutputHash        string `json:"output_hash"`
	Model             string `json:"model"`
	Decision          string `json:"decision"`
	VerifierAddress   string `json:"verifier_address"`
	VerifierPublicKey string `json:"verifier_public_key"`
	ID                string `json:"id"`
	Signature         string `json:"signature"`
}

// Fixed-order JSON payload: ID and Signature are intentionally excluded.
type quantumReportPayload struct {
	Version           uint8  `json:"version"`
	JobID             string `json:"job_id"`
	ProofID           string `json:"proof_id"`
	ChainID           string `json:"chain_id"`
	GenesisHash       string `json:"genesis_hash"`
	TaskID            string `json:"task_id"`
	OutputHash        string `json:"output_hash"`
	Model             string `json:"model"`
	Decision          string `json:"decision"`
	VerifierAddress   string `json:"verifier_address"`
	VerifierPublicKey string `json:"verifier_public_key"`
}

func (r QuantumVerificationReport) payload() quantumReportPayload {
	return quantumReportPayload{
		Version:           r.Version,
		JobID:             r.JobID,
		ProofID:           r.ProofID,
		ChainID:           r.ChainID,
		GenesisHash:       r.GenesisHash,
		TaskID:            r.TaskID,
		OutputHash:        r.OutputHash,
		Model:             r.Model,
		Decision:          r.Decision,
		VerifierAddress:   r.VerifierAddress,
		VerifierPublicKey: r.VerifierPublicKey,
	}
}

func quantumReportDigest(r QuantumVerificationReport) ([32]byte, error) {
	data, err := json.Marshal(r.payload())
	if err != nil {
		return [32]byte{}, err
	}

	preimage := append([]byte(quantumReportDomain), data...)
	return sha256.Sum256(preimage), nil
}

func quantumReportForProof(
	proof Proof,
	verifierAddress string,
	verifierPublicKey string,
) (QuantumVerificationReport, error) {

	if proof.ProofVersion != ComputeProofVersion {
		return QuantumVerificationReport{}, fmt.Errorf(
			"quantum report requires Proof v2",
		)
	}

	if proof.Task.Type != TaskTypeQuantumSimulation {
		return QuantumVerificationReport{}, fmt.Errorf(
			"quantum report requires quantum simulation",
		)
	}

	if err := VerifyProof(proof); err != nil {
		return QuantumVerificationReport{}, fmt.Errorf(
			"invalid source proof: %w", err,
		)
	}

	_, shots, err := QuantumSimulationParameters(proof.Task)
	if err != nil {
		return QuantumVerificationReport{}, err
	}

	decision := QuantumDecisionAccept
	if _, err := VerifyBellReference(proof.ResultValues, shots); err != nil {
		decision = QuantumDecisionReject
	}

	return QuantumVerificationReport{
		Version:           QuantumReportVersion,
		JobID:             proof.JobID,
		ProofID:           proof.ID,
		ChainID:           proof.ChainID,
		GenesisHash:       proof.GenesisHash,
		TaskID:            proof.Task.ID,
		OutputHash:        proof.OutputHash,
		Model:             "bell-ideal-v1",
		Decision:          decision,
		VerifierAddress:   verifierAddress,
		VerifierPublicKey: verifierPublicKey,
	}, nil
}

func SignQuantumVerificationReport(
	proof Proof,
	verifier *wallet.Wallet,
) (QuantumVerificationReport, error) {

	if verifier == nil {
		return QuantumVerificationReport{}, fmt.Errorf("nil verifier")
	}

	if len(verifier.PrivateKey) != ed25519.PrivateKeySize ||
		len(verifier.PublicKey) != ed25519.PublicKeySize {
		return QuantumVerificationReport{}, fmt.Errorf(
			"invalid verifier key pair",
		)
	}

	derivedPublic := verifier.PrivateKey.Public().(ed25519.PublicKey)
	if !bytes.Equal(derivedPublic, verifier.PublicKey) {
		return QuantumVerificationReport{}, fmt.Errorf(
			"verifier key pair mismatch",
		)
	}

	address := wallet.AddressFromPublicKey(verifier.PublicKey)
	if verifier.Address != address {
		return QuantumVerificationReport{}, fmt.Errorf(
			"verifier address mismatch",
		)
	}

	if address == proof.Worker {
		return QuantumVerificationReport{}, fmt.Errorf(
			"worker cannot self-verify",
		)
	}

	report, err := quantumReportForProof(
		proof,
		address,
		verifier.PublicKeyHex(),
	)
	if err != nil {
		return QuantumVerificationReport{}, err
	}

	digest, err := quantumReportDigest(report)
	if err != nil {
		return QuantumVerificationReport{}, err
	}

	report.ID = hex.EncodeToString(digest[:])
	report.Signature = hex.EncodeToString(
		ed25519.Sign(verifier.PrivateKey, digest[:]),
	)

	return report, nil
}

func VerifyQuantumVerificationReport(
	report QuantumVerificationReport,
	proof Proof,
) error {

	expected, err := quantumReportForProof(
		proof,
		report.VerifierAddress,
		report.VerifierPublicKey,
	)
	if err != nil {
		return err
	}

	if report.payload() != expected.payload() {
		return fmt.Errorf("quantum report payload mismatch")
	}

	publicKey, err := wallet.DecodePublicKey(report.VerifierPublicKey)
	if err != nil {
		return err
	}

	derivedAddress := wallet.AddressFromPublicKey(publicKey)
	if derivedAddress != report.VerifierAddress {
		return fmt.Errorf("quantum verifier identity mismatch")
	}

	if derivedAddress == proof.Worker {
		return fmt.Errorf("worker cannot self-verify")
	}

	digest, err := quantumReportDigest(report)
	if err != nil {
		return err
	}

	if report.ID != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("quantum report ID mismatch")
	}

	signature, err := hex.DecodeString(report.Signature)
	if err != nil {
		return fmt.Errorf("invalid quantum signature encoding: %w", err)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid quantum signature size")
	}

	if !ed25519.Verify(publicKey, digest[:], signature) {
		return fmt.Errorf("invalid quantum verification signature")
	}

	return nil
}

func VerifyQuantumVerificationAcceptance(
	report QuantumVerificationReport,
	proof Proof,
) error {
	if err := VerifyQuantumVerificationReport(report, proof); err != nil {
		return err
	}

	if report.Decision != QuantumDecisionAccept {
		return fmt.Errorf(
			"quantum reference verification did not accept proof: %s",
			report.Decision,
		)
	}

	return nil
}
