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

type Approval struct {
	Authorizer string `json:"authorizer"`
	PublicKey  string `json:"public_key"`
	Signature  string `json:"signature"`
}

type Grant struct {
	ID string `json:"id"`

	ChainID string `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`

	Pool      consensus.ReservedPool `json:"pool"`
	Recipient string                 `json:"recipient"`
	Amount    uint64                 `json:"amount"`

	Approvals []Approval `json:"approvals,omitempty"`
}

func NewGrant(
	chainID string,
	nonce uint64,
	pool consensus.ReservedPool,
	recipient string,
	amount uint64,
) Grant {
	grant := Grant{
		ChainID:   chainID,
		Nonce:     nonce,
		Pool:      pool,
		Recipient: recipient,
		Amount:    amount,
		Approvals: []Approval{},
	}

	grant.ID = CalculateGrantID(grant)

	return grant
}

func grantPayload(
	grant Grant,
) string {
	return fmt.Sprintf(
		"reserved-grant-v1|%s|%d|%s|%s|%d",
		grant.ChainID,
		grant.Nonce,
		grant.Pool,
		grant.Recipient,
		grant.Amount,
	)
}

func CalculateGrantID(
	grant Grant,
) string {
	hash := sha256.Sum256(
		[]byte(
			grantPayload(grant),
		),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

func ValidateGrant(
	grant Grant,
) error {
	if grant.ChainID == "" {
		return fmt.Errorf(
			"reserved grant chain ID cannot be empty",
		)
	}

	if grant.Nonce == 0 {
		return fmt.Errorf(
			"reserved grant nonce must be greater than zero",
		)
	}

	policy := consensus.DefaultSupplyPolicy()

	allocation, err :=
		policy.ReservedPoolAllocation(
			grant.Pool,
		)

	if err != nil {
		return err
	}

	if grant.Recipient == "" {
		return fmt.Errorf(
			"reserved grant recipient cannot be empty",
		)
	}

	if grant.Recipient == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot receive reserved grant",
		)
	}

	if grant.Amount == 0 {
		return fmt.Errorf(
			"reserved grant amount must be greater than zero",
		)
	}

	if grant.Amount > allocation {
		return fmt.Errorf(
			"reserved grant amount exceeds pool allocation",
		)
	}

	if grant.ID == "" {
		return fmt.Errorf(
			"reserved grant ID cannot be empty",
		)
	}

	if grant.ID != CalculateGrantID(grant) {
		return fmt.Errorf(
			"invalid reserved grant ID",
		)
	}

	return nil
}

func (grant *Grant) AddApproval(
	authorizer string,
	publicKey string,
	privateKey ed25519.PrivateKey,
) error {
	if grant == nil {
		return fmt.Errorf(
			"reserved grant cannot be nil",
		)
	}

	if err := ValidateGrant(*grant); err != nil {
		return err
	}

	if authorizer == "" {
		return fmt.Errorf(
			"reserved approval authorizer cannot be empty",
		)
	}

	if authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved grant",
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

	for _, approval := range grant.Approvals {
		if approval.Authorizer == authorizer {
			return fmt.Errorf(
				"reserved grant already approved by authorizer",
			)
		}
	}

	signature := ed25519.Sign(
		privateKey,
		[]byte(grant.ID),
	)

	grant.Approvals = append(
		grant.Approvals,
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

func ValidateApproval(
	grant Grant,
	approval Approval,
) error {
	if err := ValidateGrant(grant); err != nil {
		return err
	}

	if approval.Authorizer == "" {
		return fmt.Errorf(
			"reserved approval authorizer cannot be empty",
		)
	}

	if approval.Authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved grant",
		)
	}

	if approval.PublicKey == "" {
		return fmt.Errorf(
			"reserved approval public key cannot be empty",
		)
	}

	if approval.Signature == "" {
		return fmt.Errorf(
			"reserved approval signature cannot be empty",
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

	if approval.Authorizer != expectedAuthorizer {
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
			"invalid reserved approval signature encoding: %w",
			err,
		)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf(
			"invalid reserved approval signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(grant.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid reserved approval signature",
		)
	}

	return nil
}
