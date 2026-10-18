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

// ComputeVerificationReportV064 is an optional off-chain attestation that a
// verifier independently re-executed a quantized ML proof. It is NOT a ZK proof,
// a hardware attestation, a consensus vote, or authorization to pay a bounty.
const (
	ComputeVerificationVersionV064 uint8 = 1
	computeVerificationDomainV064        = "Prism/PoUW/IndependentVerification/v0.64\x00"
)

type ComputeVerificationReportV064 struct {
	Version           uint8  `json:"version"`
	JobID             string `json:"job_id"`
	ProofID           string `json:"proof_id"`
	ChainID           string `json:"chain_id"`
	GenesisHash       string `json:"genesis_hash"`
	TaskID            string `json:"task_id"`
	InputHash         string `json:"input_hash"`
	OutputHash        string `json:"output_hash"`
	WorkUnits         uint64 `json:"work_units"`
	VerifierAddress   string `json:"verifier_address"`
	VerifierPublicKey string `json:"verifier_public_key"`
	ID                string `json:"id"`
	Signature         string `json:"signature"`
}

// Fixed-order JSON deliberately excludes ID and Signature.
type computeVerificationPayloadV064 struct {
	Version           uint8  `json:"version"`
	JobID             string `json:"job_id"`
	ProofID           string `json:"proof_id"`
	ChainID           string `json:"chain_id"`
	GenesisHash       string `json:"genesis_hash"`
	TaskID            string `json:"task_id"`
	InputHash         string `json:"input_hash"`
	OutputHash        string `json:"output_hash"`
	WorkUnits         uint64 `json:"work_units"`
	VerifierAddress   string `json:"verifier_address"`
	VerifierPublicKey string `json:"verifier_public_key"`
}

func (r ComputeVerificationReportV064) payload() computeVerificationPayloadV064 {
	return computeVerificationPayloadV064{
		Version:           r.Version,
		JobID:             r.JobID,
		ProofID:           r.ProofID,
		ChainID:           r.ChainID,
		GenesisHash:       r.GenesisHash,
		TaskID:            r.TaskID,
		InputHash:         r.InputHash,
		OutputHash:        r.OutputHash,
		WorkUnits:         r.WorkUnits,
		VerifierAddress:   r.VerifierAddress,
		VerifierPublicKey: r.VerifierPublicKey,
	}
}

func computeVerificationDigestV064(r ComputeVerificationReportV064) ([32]byte, error) {
	data, err := json.Marshal(r.payload())
	if err != nil {
		return [32]byte{}, err
	}
	preimage := make([]byte, 0, len(computeVerificationDomainV064)+len(data))
	preimage = append(preimage, computeVerificationDomainV064...)
	preimage = append(preimage, data...)
	return sha256.Sum256(preimage), nil
}

func expectedComputeVerificationV064(
	proof Proof,
	verifierAddress string,
	verifierPublicKey string,
) (ComputeVerificationReportV064, error) {
	if proof.ProofVersion != ComputeProofVersion {
		return ComputeVerificationReportV064{}, fmt.Errorf("v0.64 verifier requires context-bound PoUW Proof v2")
	}
	if proof.Task.Type != TaskTypeMLInferenceQuantized {
		return ComputeVerificationReportV064{}, fmt.Errorf("v0.64 verifier only supports quantized ML inference")
	}
	if proof.JobID == "" || proof.ChainID == "" || proof.GenesisHash == "" {
		return ComputeVerificationReportV064{}, fmt.Errorf("missing signed proof context")
	}
	// VerifyProof independently recomputes the complete ML result on CPU,
	// validates input/output commitments, work score, identity and signature.
	if err := VerifyProof(proof); err != nil {
		return ComputeVerificationReportV064{}, fmt.Errorf("invalid source PoUW proof: %w", err)
	}
	workUnits, err := WorkUnits(proof.Task)
	if err != nil {
		return ComputeVerificationReportV064{}, err
	}
	return ComputeVerificationReportV064{
		Version:           ComputeVerificationVersionV064,
		JobID:             proof.JobID,
		ProofID:           proof.ID,
		ChainID:           proof.ChainID,
		GenesisHash:       proof.GenesisHash,
		TaskID:            proof.Task.ID,
		InputHash:         proof.Task.InputHash,
		OutputHash:        proof.OutputHash,
		WorkUnits:         workUnits,
		VerifierAddress:   verifierAddress,
		VerifierPublicKey: verifierPublicKey,
	}, nil
}

func SignComputeVerificationV064(
	proof Proof,
	verifier *wallet.Wallet,
) (ComputeVerificationReportV064, error) {
	if verifier == nil {
		return ComputeVerificationReportV064{}, fmt.Errorf("nil independent verifier")
	}
	if len(verifier.PublicKey) != ed25519.PublicKeySize ||
		len(verifier.PrivateKey) != ed25519.PrivateKeySize {
		return ComputeVerificationReportV064{}, fmt.Errorf("invalid verifier key sizes")
	}
	if !bytes.Equal(verifier.PrivateKey.Public().(ed25519.PublicKey), verifier.PublicKey) {
		return ComputeVerificationReportV064{}, fmt.Errorf("verifier keypair mismatch")
	}
	if wallet.AddressFromPublicKey(verifier.PublicKey) != verifier.Address {
		return ComputeVerificationReportV064{}, fmt.Errorf("verifier address mismatch")
	}
	if verifier.Address == proof.Worker {
		return ComputeVerificationReportV064{}, fmt.Errorf("worker cannot self-verify")
	}

	report, err := expectedComputeVerificationV064(
		proof, verifier.Address, verifier.PublicKeyHex(),
	)
	if err != nil {
		return ComputeVerificationReportV064{}, err
	}
	digest, err := computeVerificationDigestV064(report)
	if err != nil {
		return ComputeVerificationReportV064{}, err
	}
	report.ID = hex.EncodeToString(digest[:])
	report.Signature = hex.EncodeToString(ed25519.Sign(verifier.PrivateKey, digest[:]))
	return report, nil
}

func VerifyComputeVerificationV064(
	report ComputeVerificationReportV064,
	proof Proof,
) error {
	expected, err := expectedComputeVerificationV064(
		proof, report.VerifierAddress, report.VerifierPublicKey,
	)
	if err != nil {
		return err
	}
	if report.payload() != expected.payload() {
		return fmt.Errorf("v0.64 verification report does not match proof")
	}
	publicKey, err := wallet.DecodePublicKey(report.VerifierPublicKey)
	if err != nil {
		return err
	}
	if wallet.AddressFromPublicKey(publicKey) != report.VerifierAddress {
		return fmt.Errorf("independent verifier identity mismatch")
	}
	if report.VerifierAddress == proof.Worker {
		return fmt.Errorf("worker cannot self-verify")
	}
	digest, err := computeVerificationDigestV064(report)
	if err != nil {
		return err
	}
	if report.ID != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("v0.64 report digest mismatch")
	}
	signature, err := hex.DecodeString(report.Signature)
	if err != nil {
		return fmt.Errorf("invalid v0.64 report signature encoding: %w", err)
	}
	if len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(publicKey, digest[:], signature) {
		return fmt.Errorf("invalid v0.64 verifier signature")
	}
	return nil
}
