package crosschain

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/sha3"
)

const ReceiptVersion uint8 = 1

type Receipt struct {
	Version          uint8  `json:"version"`
	JobID            string `json:"jobId"`
	ProofID          string `json:"proofId"`
	WorkerIDHash     string `json:"workerIdHash"`
	PrismChainIDHash string `json:"prismChainIdHash"`
	RegistryID       string `json:"registryId"`
}

func NewReceipt(
	jobID string,
	proofID string,
	workerID string,
	prismChainID string,
) (Receipt, error) {
	jobBytes, err := decodeBytes32(jobID)
	if err != nil {
		return Receipt{}, fmt.Errorf(
			"invalid cross-chain job ID: %w",
			err,
		)
	}

	proofBytes, err := decodeBytes32(proofID)
	if err != nil {
		return Receipt{}, fmt.Errorf(
			"invalid cross-chain proof ID: %w",
			err,
		)
	}

	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return Receipt{}, fmt.Errorf(
			"cross-chain worker ID cannot be empty",
		)
	}

	prismChainID = strings.TrimSpace(prismChainID)
	if prismChainID == "" {
		return Receipt{}, fmt.Errorf(
			"cross-chain Prism chain ID cannot be empty",
		)
	}

	workerHash := keccak256([]byte(workerID))
	chainHash := keccak256([]byte(prismChainID))

	registryID := keccak256(
		jobBytes,
		chainHash[:],
		proofBytes,
	)

	return Receipt{
		Version:          ReceiptVersion,
		JobID:            hex32(jobBytes),
		ProofID:          hex32(proofBytes),
		WorkerIDHash:     hex32(workerHash[:]),
		PrismChainIDHash: hex32(chainHash[:]),
		RegistryID:       hex32(registryID[:]),
	}, nil
}

func decodeBytes32(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "0x")
	value = strings.TrimPrefix(value, "0X")

	if len(value) != 64 {
		return nil, fmt.Errorf(
			"expected 32-byte hexadecimal value",
		)
	}

	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid hexadecimal value: %w",
			err,
		)
	}

	return decoded, nil
}

func keccak256(parts ...[]byte) [32]byte {
	hash := sha3.NewLegacyKeccak256()

	for _, part := range parts {
		_, _ = hash.Write(part)
	}

	var result [32]byte
	copy(result[:], hash.Sum(nil))

	return result
}

func hex32(value []byte) string {
	return "0x" + hex.EncodeToString(value)
}
