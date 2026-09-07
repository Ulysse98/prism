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

type Authorization struct {
	ID string `json:"id"`

	ChainID string `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`

	Pool      consensus.ReservedPool `json:"pool"`
	Recipient string                 `json:"recipient"`
	Amount    uint64                 `json:"amount"`

	Authorizer string `json:"authorizer"`
	PublicKey  string `json:"public_key"`
	Signature  string `json:"signature"`
}

func NewAuthorization(
	chainID string,
	nonce uint64,
	pool consensus.ReservedPool,
	recipient string,
	amount uint64,
	authorizer string,
	publicKey string,
) Authorization {
	authorization := Authorization{
		ChainID:    chainID,
		Nonce:      nonce,
		Pool:       pool,
		Recipient:  recipient,
		Amount:     amount,
		Authorizer: authorizer,
		PublicKey:  publicKey,
	}

	authorization.ID =
		CalculateID(authorization)

	return authorization
}

func authorizationPayload(
	authorization Authorization,
) string {
	return fmt.Sprintf(
		"reserved-v1|%s|%d|%s|%s|%d|%s|%s",
		authorization.ChainID,
		authorization.Nonce,
		authorization.Pool,
		authorization.Recipient,
		authorization.Amount,
		authorization.Authorizer,
		authorization.PublicKey,
	)
}

func CalculateID(
	authorization Authorization,
) string {
	hash := sha256.Sum256(
		[]byte(
			authorizationPayload(
				authorization,
			),
		),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

func (authorization *Authorization) Sign(
	privateKey ed25519.PrivateKey,
) error {
	if authorization == nil {
		return fmt.Errorf(
			"reserved authorization cannot be nil",
		)
	}

	publicKey, err :=
		wallet.DecodePublicKey(
			authorization.PublicKey,
		)

	if err != nil {
		return err
	}

	signerPublicKey :=
		privateKey.Public().(ed25519.PublicKey)

	if !bytes.Equal(
		publicKey,
		signerPublicKey,
	) {
		return fmt.Errorf(
			"private key does not match authorization public key",
		)
	}

	expectedAuthorizer :=
		wallet.AddressFromPublicKey(
			publicKey,
		)

	if authorization.Authorizer !=
		expectedAuthorizer {

		return fmt.Errorf(
			"authorization public key does not own authorizer address",
		)
	}

	authorization.ID =
		CalculateID(*authorization)

	signature := ed25519.Sign(
		privateKey,
		[]byte(authorization.ID),
	)

	authorization.Signature =
		hex.EncodeToString(signature)

	return nil
}

func ValidateSigned(
	authorization Authorization,
) error {
	if authorization.ChainID == "" {
		return fmt.Errorf(
			"reserved authorization chain ID cannot be empty",
		)
	}

	if authorization.Nonce == 0 {
		return fmt.Errorf(
			"reserved authorization nonce must be greater than zero",
		)
	}

	policy :=
		consensus.DefaultSupplyPolicy()

	allocation, err :=
		policy.ReservedPoolAllocation(
			authorization.Pool,
		)

	if err != nil {
		return err
	}

	if authorization.Recipient == "" {
		return fmt.Errorf(
			"reserved recipient cannot be empty",
		)
	}

	if authorization.Recipient == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot receive reserved emission",
		)
	}

	if authorization.Amount == 0 {
		return fmt.Errorf(
			"reserved amount must be greater than zero",
		)
	}

	if authorization.Amount > allocation {
		return fmt.Errorf(
			"reserved amount exceeds pool allocation",
		)
	}

	if authorization.Authorizer == "" {
		return fmt.Errorf(
			"reserved authorizer cannot be empty",
		)
	}

	if authorization.Authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot authorize reserved emission",
		)
	}

	if authorization.PublicKey == "" {
		return fmt.Errorf(
			"reserved authorization public key cannot be empty",
		)
	}

	if authorization.Signature == "" {
		return fmt.Errorf(
			"reserved authorization signature cannot be empty",
		)
	}

	if authorization.ID !=
		CalculateID(authorization) {

		return fmt.Errorf(
			"invalid reserved authorization ID",
		)
	}

	publicKey, err :=
		wallet.DecodePublicKey(
			authorization.PublicKey,
		)

	if err != nil {
		return err
	}

	expectedAuthorizer :=
		wallet.AddressFromPublicKey(
			publicKey,
		)

	if authorization.Authorizer !=
		expectedAuthorizer {

		return fmt.Errorf(
			"authorization public key does not own authorizer address",
		)
	}

	signature, err :=
		hex.DecodeString(
			authorization.Signature,
		)

	if err != nil {
		return fmt.Errorf(
			"invalid reserved authorization signature encoding: %w",
			err,
		)
	}

	if len(signature) !=
		ed25519.SignatureSize {

		return fmt.Errorf(
			"invalid reserved authorization signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(authorization.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid reserved authorization signature",
		)
	}

	return nil
}

func ValidateSignedForChain(
	authorization Authorization,
	expectedChainID string,
) error {
	if expectedChainID == "" {
		return fmt.Errorf(
			"expected chain ID cannot be empty",
		)
	}

	if err :=
		ValidateSigned(
			authorization,
		); err != nil {

		return err
	}

	if authorization.ChainID !=
		expectedChainID {

		return fmt.Errorf(
			"reserved authorization chain ID mismatch",
		)
	}

	return nil
}
