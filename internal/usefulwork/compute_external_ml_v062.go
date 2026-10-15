package usefulwork

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"prism/internal/wallet"
)

// BuildComputeMLProofV2FromResult signs externally computed
// ML predictions without recomputing inference locally.
//
// It validates dimensions, class indices and commitments,
// but does NOT establish that predictions are correct.
// Consensus verification remains responsible for that.
func BuildComputeMLProofV2FromResult(
	task Task,
	jobID string,
	chainID string,
	genesisHash string,
	worker *wallet.Wallet,
	predictions []uint64,
) (Proof, error) {
	if worker == nil {
		return Proof{}, fmt.Errorf("worker wallet is nil")
	}

	if len(worker.PublicKey) != ed25519.PublicKeySize ||
		len(worker.PrivateKey) != ed25519.PrivateKeySize {
		return Proof{}, fmt.Errorf("invalid worker key sizes")
	}

	if !bytes.Equal(
		worker.PublicKey,
		worker.PrivateKey[32:],
	) {
		return Proof{}, fmt.Errorf("worker keypair mismatch")
	}

	if wallet.AddressFromPublicKey(worker.PublicKey) !=
		worker.Address {
		return Proof{}, fmt.Errorf("worker address mismatch")
	}

	if jobID == "" || chainID == "" || genesisHash == "" {
		return Proof{}, fmt.Errorf(
			"missing compute proof context",
		)
	}

	if err := ValidateTask(task); err != nil {
		return Proof{}, err
	}

	if task.Type != TaskTypeMLInferenceQuantized {
		return Proof{}, fmt.Errorf(
			"unsupported external ML task type: %s",
			task.Type,
		)
	}

	if uint64(len(predictions)) != task.RowsA {
		return Proof{}, fmt.Errorf(
			"prediction count mismatch",
		)
	}

	for _, class := range predictions {
		if class >= task.ColsB {
			return Proof{}, fmt.Errorf(
				"prediction class out of range",
			)
		}
	}

	values := append([]uint64(nil), predictions...)

	outputHash, err := calculateValuesOutputHash(values)
	if err != nil {
		return Proof{}, err
	}

	proof := Proof{
		ProofVersion: ComputeProofVersion,
		JobID:        jobID,
		ChainID:      chainID,
		GenesisHash:  genesisHash,
		Task:         task,
		Worker:       worker.Address,
		PublicKey:    worker.PublicKeyHex(),
		ResultValues: values,
		OutputHash:   outputHash,
		Score:        scoreForTask(task),
	}

	proof.ID = CalculateComputeProofID(proof)

	proof.Signature = hex.EncodeToString(
		ed25519.Sign(worker.PrivateKey, []byte(proof.ID)),
	)

	return proof, nil
}
