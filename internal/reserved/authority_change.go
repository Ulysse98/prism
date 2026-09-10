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

type AuthorityChangeAction string

const (
	AuthorityChangeAdd    AuthorityChangeAction = "add"
	AuthorityChangeRemove AuthorityChangeAction = "remove"
)

type AuthorityChange struct {
	ID string `json:"id"`

	ChainID string `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`

	Pool      consensus.ReservedPool `json:"pool"`
	Action    AuthorityChangeAction  `json:"action"`
	Authority string                 `json:"authority"`

	Approvals []Approval `json:"approvals,omitempty"`
}

func NewAuthorityChange(
	chainID string,
	nonce uint64,
	pool consensus.ReservedPool,
	action AuthorityChangeAction,
	authority string,
) AuthorityChange {
	change := AuthorityChange{
		ChainID:   chainID,
		Nonce:     nonce,
		Pool:      pool,
		Action:    action,
		Authority: authority,
		Approvals: []Approval{},
	}

	change.ID =
		CalculateAuthorityChangeID(change)

	return change
}

func authorityChangePayload(
	change AuthorityChange,
) string {
	return fmt.Sprintf(
		"reserved-authority-change-v1|%s|%d|%s|%s|%s",
		change.ChainID,
		change.Nonce,
		change.Pool,
		change.Action,
		change.Authority,
	)
}

func CalculateAuthorityChangeID(
	change AuthorityChange,
) string {
	hash := sha256.Sum256(
		[]byte(
			authorityChangePayload(change),
		),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

func ValidateAuthorityChange(
	change AuthorityChange,
) error {
	if change.ChainID == "" {
		return fmt.Errorf(
			"reserved authority change chain ID cannot be empty",
		)
	}

	if change.Nonce == 0 {
		return fmt.Errorf(
			"reserved authority change nonce must be greater than zero",
		)
	}

	supplyPolicy :=
		consensus.DefaultSupplyPolicy()

	if _, err :=
		supplyPolicy.ReservedPoolAllocation(
			change.Pool,
		); err != nil {

		return err
	}

	switch change.Action {
	case AuthorityChangeAdd:
	case AuthorityChangeRemove:
	default:
		return fmt.Errorf(
			"invalid reserved authority change action: %q",
			change.Action,
		)
	}

	if change.Authority == "" {
		return fmt.Errorf(
			"reserved authority change target cannot be empty",
		)
	}

	if change.Authority == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot be a reserved authority",
		)
	}

	if change.ID == "" {
		return fmt.Errorf(
			"reserved authority change ID cannot be empty",
		)
	}

	if change.ID !=
		CalculateAuthorityChangeID(change) {

		return fmt.Errorf(
			"invalid reserved authority change ID",
		)
	}

	return nil
}

func (change *AuthorityChange) AddApproval(
	authorizer string,
	publicKey string,
	privateKey ed25519.PrivateKey,
) error {
	if change == nil {
		return fmt.Errorf(
			"reserved authority change cannot be nil",
		)
	}

	if err :=
		ValidateAuthorityChange(*change); err != nil {

		return err
	}

	if authorizer == "" {
		return fmt.Errorf(
			"reserved authority change approval authorizer cannot be empty",
		)
	}

	if authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved authority change",
		)
	}

	if len(privateKey) !=
		ed25519.PrivateKeySize {

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

	for _, approval := range change.Approvals {

		if approval.Authorizer ==
			authorizer {

			return fmt.Errorf(
				"reserved authority change already approved by authorizer",
			)
		}
	}

	signature :=
		ed25519.Sign(
			privateKey,
			[]byte(change.ID),
		)

	change.Approvals =
		append(
			change.Approvals,
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

func ValidateAuthorityChangeApproval(
	change AuthorityChange,
	approval Approval,
) error {
	if err :=
		ValidateAuthorityChange(change); err != nil {

		return err
	}

	if approval.Authorizer == "" {
		return fmt.Errorf(
			"reserved authority change approval authorizer cannot be empty",
		)
	}

	if approval.Authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved authority change",
		)
	}

	if approval.PublicKey == "" {
		return fmt.Errorf(
			"reserved authority change approval public key cannot be empty",
		)
	}

	if approval.Signature == "" {
		return fmt.Errorf(
			"reserved authority change approval signature cannot be empty",
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
			"invalid reserved authority change approval signature encoding: %w",
			err,
		)
	}

	if len(signature) !=
		ed25519.SignatureSize {

		return fmt.Errorf(
			"invalid reserved authority change approval signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(change.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid reserved authority change approval signature",
		)
	}

	return nil
}
