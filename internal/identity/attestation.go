package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const WorldIDProvider = "World ID"

type Attestation struct {
	Address       string `json:"address"`
	Provider      string `json:"provider"`
	Action        string `json:"action"`
	NullifierHash string `json:"nullifier_hash"`
}

// NewWorldIDAttestation converts a locally verified World ID
// result into the minimal data Prism needs on-chain.
//
// The raw nullifier is never stored in the blockchain.
func NewWorldIDAttestation(
	address string,
	nullifier string,
	action string,
) (Attestation, error) {
	if address == "" {
		return Attestation{}, fmt.Errorf(
			"attestation address cannot be empty",
		)
	}

	if nullifier == "" {
		return Attestation{}, fmt.Errorf(
			"attestation nullifier cannot be empty",
		)
	}

	if action == "" {
		return Attestation{}, fmt.Errorf(
			"attestation action cannot be empty",
		)
	}

	sum := sha256.Sum256([]byte(nullifier))

	return Attestation{
		Address:       address,
		Provider:      WorldIDProvider,
		Action:        action,
		NullifierHash: hex.EncodeToString(sum[:]),
	}, nil
}

func ValidateAttestation(attestation Attestation) error {
	if attestation.Address == "" {
		return fmt.Errorf(
			"attestation address cannot be empty",
		)
	}

	if attestation.Provider != WorldIDProvider {
		return fmt.Errorf(
			"unsupported humanity provider: %s",
			attestation.Provider,
		)
	}

	if attestation.Action == "" {
		return fmt.Errorf(
			"attestation action cannot be empty",
		)
	}

	if len(attestation.NullifierHash) != 64 {
		return fmt.Errorf(
			"invalid nullifier hash length",
		)
	}

	if _, err := hex.DecodeString(
		attestation.NullifierHash,
	); err != nil {
		return fmt.Errorf(
			"invalid nullifier hash: %w",
			err,
		)
	}

	return nil
}
