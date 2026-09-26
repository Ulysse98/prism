package compute

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"prism/internal/wallet"
)

const ClaimAuthorizationVersion uint8 = 1

type ClaimAuthorization struct {
	ID          string `json:"claimId"`
	Version     uint8  `json:"version"`
	JobID       string `json:"jobId"`
	ChainID     string `json:"chainId"`
	GenesisHash string `json:"genesisHash"`
	Worker      string `json:"worker"`
	PublicKey   string `json:"publicKey"`
	Signature   string `json:"signature"`
}

func claimAuthorizationPayload(
	claim ClaimAuthorization,
) string {
	return fmt.Sprintf(
		"Prism/Compute/Claim/v1|%s|%s|%s|%s|%s",
		claim.JobID,
		claim.ChainID,
		claim.GenesisHash,
		claim.Worker,
		claim.PublicKey,
	)
}

func CalculateClaimAuthorizationID(
	claim ClaimAuthorization,
) string {
	hash := sha256.Sum256(
		[]byte(
			claimAuthorizationPayload(claim),
		),
	)

	return hex.EncodeToString(hash[:])
}

func validateClaimContextValue(
	label string,
	value string,
) (string, error) {
	trimmed := strings.TrimSpace(value)

	if trimmed == "" {
		return "", fmt.Errorf(
			"%s cannot be empty",
			label,
		)
	}

	if trimmed != value {
		return "", fmt.Errorf(
			"%s cannot contain surrounding whitespace",
			label,
		)
	}

	return value, nil
}

func SignClaimAuthorization(
	jobID string,
	chainID string,
	genesisHash string,
	worker *wallet.Wallet,
) (ClaimAuthorization, error) {
	if worker == nil {
		return ClaimAuthorization{},
			fmt.Errorf(
				"compute claim worker wallet cannot be nil",
			)
	}

	var err error

	jobID, err = validateClaimContextValue(
		"compute claim job ID",
		jobID,
	)
	if err != nil {
		return ClaimAuthorization{}, err
	}

	chainID, err = validateClaimContextValue(
		"compute claim chain ID",
		chainID,
	)
	if err != nil {
		return ClaimAuthorization{}, err
	}

	genesisHash, err = validateClaimContextValue(
		"compute claim genesis hash",
		genesisHash,
	)
	if err != nil {
		return ClaimAuthorization{}, err
	}

	claim := ClaimAuthorization{
		Version:     ClaimAuthorizationVersion,
		JobID:       jobID,
		ChainID:     chainID,
		GenesisHash: genesisHash,
		Worker:      worker.Address,
		PublicKey:   worker.PublicKeyHex(),
	}

	claim.ID = CalculateClaimAuthorizationID(
		claim,
	)

	claim.Signature = hex.EncodeToString(
		ed25519.Sign(
			worker.PrivateKey,
			[]byte(claim.ID),
		),
	)

	return claim, nil
}

func VerifyClaimAuthorization(
	claim ClaimAuthorization,
	expectedJobID string,
	expectedChainID string,
	expectedGenesisHash string,
) error {
	if claim.Version != ClaimAuthorizationVersion {
		return fmt.Errorf(
			"unsupported compute claim version: %d",
			claim.Version,
		)
	}

	if claim.JobID != expectedJobID {
		return fmt.Errorf(
			"compute claim job ID does not match endpoint",
		)
	}

	if claim.ChainID != expectedChainID {
		return fmt.Errorf(
			"compute claim chain ID does not match node",
		)
	}

	if claim.GenesisHash != expectedGenesisHash {
		return fmt.Errorf(
			"compute claim genesis hash does not match node",
		)
	}

	if strings.TrimSpace(claim.Worker) == "" ||
		strings.TrimSpace(claim.Worker) != claim.Worker {

		return fmt.Errorf(
			"compute claim worker is invalid",
		)
	}

	if strings.TrimSpace(claim.PublicKey) == "" ||
		strings.TrimSpace(claim.PublicKey) != claim.PublicKey {

		return fmt.Errorf(
			"compute claim public key is invalid",
		)
	}

	publicKey, err := wallet.DecodePublicKey(
		claim.PublicKey,
	)
	if err != nil {
		return fmt.Errorf(
			"invalid compute claim public key: %w",
			err,
		)
	}

	if wallet.AddressFromPublicKey(
		publicKey,
	) != claim.Worker {

		return fmt.Errorf(
			"compute claim public key does not own worker address",
		)
	}

	expectedID := CalculateClaimAuthorizationID(
		claim,
	)

	if claim.ID != expectedID {
		return fmt.Errorf(
			"invalid compute claim ID",
		)
	}

	signature, err := hex.DecodeString(
		claim.Signature,
	)
	if err != nil {
		return fmt.Errorf(
			"invalid compute claim signature encoding: %w",
			err,
		)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf(
			"invalid compute claim signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(claim.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid compute claim signature",
		)
	}

	return nil
}
