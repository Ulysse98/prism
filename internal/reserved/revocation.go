package reserved

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

type Revocation struct {
	ID string `json:"id"`

	ChainID string                 `json:"chain_id"`
	Pool    consensus.ReservedPool `json:"pool"`
	GrantID string                 `json:"grant_id"`

	Approvals []Approval `json:"approvals,omitempty"`
}

func NewRevocation(
	chainID string,
	pool consensus.ReservedPool,
	grantID string,
) Revocation {
	revocation := Revocation{
		ChainID:   chainID,
		Pool:      pool,
		GrantID:   grantID,
		Approvals: []Approval{},
	}

	revocation.ID =
		CalculateRevocationID(revocation)

	return revocation
}

func revocationPayload(
	revocation Revocation,
) string {
	return fmt.Sprintf(
		"reserved-grant-revocation-v1|%s|%s|%s",
		revocation.ChainID,
		revocation.Pool,
		revocation.GrantID,
	)
}

func CalculateRevocationID(
	revocation Revocation,
) string {
	hash := sha256.Sum256(
		[]byte(
			revocationPayload(revocation),
		),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

func ValidateRevocation(
	revocation Revocation,
) error {
	if revocation.ChainID == "" {
		return fmt.Errorf(
			"reserved revocation chain ID cannot be empty",
		)
	}

	policy := consensus.DefaultSupplyPolicy()

	if _, err :=
		policy.ReservedPoolAllocation(
			revocation.Pool,
		); err != nil {

		return err
	}

	if revocation.GrantID == "" {
		return fmt.Errorf(
			"reserved revocation grant ID cannot be empty",
		)
	}

	grantID, err :=
		hex.DecodeString(
			revocation.GrantID,
		)

	if err != nil {
		return fmt.Errorf(
			"invalid reserved revocation grant ID encoding: %w",
			err,
		)
	}

	if len(grantID) != sha256.Size {
		return fmt.Errorf(
			"invalid reserved revocation grant ID size",
		)
	}

	if revocation.ID == "" {
		return fmt.Errorf(
			"reserved revocation ID cannot be empty",
		)
	}

	if revocation.ID !=
		CalculateRevocationID(revocation) {

		return fmt.Errorf(
			"invalid reserved revocation ID",
		)
	}

	return nil
}

func (revocation *Revocation) AddApproval(
	authorizer string,
	publicKey string,
	privateKey ed25519.PrivateKey,
) error {
	if revocation == nil {
		return fmt.Errorf(
			"reserved revocation cannot be nil",
		)
	}

	if err :=
		ValidateRevocation(*revocation); err != nil {

		return err
	}

	if authorizer == "" {
		return fmt.Errorf(
			"reserved revocation approval authorizer cannot be empty",
		)
	}

	if authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved revocation",
		)
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf(
			"invalid approval private key size",
		)
	}

	decodedPublicKey, err :=
		wallet.DecodePublicKey(
			publicKey,
		)

	if err != nil {
		return err
	}

	signerPublicKey :=
		privateKey.Public().(ed25519.PublicKey)

	if !bytes.Equal(
		decodedPublicKey,
		signerPublicKey,
	) {
		return fmt.Errorf(
			"private key does not match approval public key",
		)
	}

	expectedAuthorizer :=
		wallet.AddressFromPublicKey(
			decodedPublicKey,
		)

	if authorizer != expectedAuthorizer {
		return fmt.Errorf(
			"approval public key does not own authorizer address",
		)
	}

	for _, approval := range revocation.Approvals {

		if approval.Authorizer == authorizer {
			return fmt.Errorf(
				"reserved revocation already approved by authorizer",
			)
		}
	}

	signature := ed25519.Sign(
		privateKey,
		[]byte(revocation.ID),
	)

	revocation.Approvals =
		append(
			revocation.Approvals,
			Approval{
				Authorizer: authorizer,
				PublicKey:  publicKey,
				Signature: hex.EncodeToString(
					signature,
				),
			},
		)

	return nil
}

func ValidateRevocationApproval(
	revocation Revocation,
	approval Approval,
) error {
	if err :=
		ValidateRevocation(revocation); err != nil {

		return err
	}

	if approval.Authorizer == "" {
		return fmt.Errorf(
			"reserved revocation approval authorizer cannot be empty",
		)
	}

	if approval.Authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved revocation",
		)
	}

	publicKey, err :=
		wallet.DecodePublicKey(
			approval.PublicKey,
		)

	if err != nil {
		return err
	}

	expectedAuthorizer :=
		wallet.AddressFromPublicKey(
			publicKey,
		)

	if approval.Authorizer !=
		expectedAuthorizer {

		return fmt.Errorf(
			"approval public key does not own authorizer address",
		)
	}

	signature, err :=
		hex.DecodeString(
			approval.Signature,
		)

	if err != nil {
		return fmt.Errorf(
			"invalid reserved revocation approval signature encoding: %w",
			err,
		)
	}

	if len(signature) !=
		ed25519.SignatureSize {

		return fmt.Errorf(
			"invalid reserved revocation approval signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(revocation.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid reserved revocation approval signature",
		)
	}

	return nil
}
