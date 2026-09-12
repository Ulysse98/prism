package reserved

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"prism/internal/wallet"
)

// AddApproval signs the immutable reserved transfer proposal ID.
//
// Approvals are deliberately excluded from the proposal ID so that
// multiple authorities can sign the same governance intent.
func (proposal *ReservedTransferProposal) AddApproval(
	authorizer string,
	publicKey string,
	privateKey ed25519.PrivateKey,
) error {
	if proposal == nil {
		return fmt.Errorf(
			"reserved transfer proposal cannot be nil",
		)
	}

	if err :=
		ValidateReservedTransferProposal(
			*proposal,
		); err != nil {

		return err
	}

	if authorizer == "" {
		return fmt.Errorf(
			"reserved transfer proposal approval authorizer cannot be empty",
		)
	}

	if authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved transfer proposal",
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

	for _, approval := range proposal.Approvals {

		if approval.Authorizer ==
			authorizer {

			return fmt.Errorf(
				"reserved transfer proposal already approved by authorizer",
			)
		}
	}

	signature :=
		ed25519.Sign(
			privateKey,
			[]byte(proposal.ID),
		)

	proposal.Approvals =
		append(
			proposal.Approvals,
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

func ValidateReservedTransferProposalApproval(
	proposal ReservedTransferProposal,
	approval Approval,
) error {
	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err != nil {

		return err
	}

	if approval.Authorizer == "" {
		return fmt.Errorf(
			"reserved transfer proposal approval authorizer cannot be empty",
		)
	}

	if approval.Authorizer == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot approve reserved transfer proposal",
		)
	}

	if approval.PublicKey == "" {
		return fmt.Errorf(
			"reserved transfer proposal approval public key cannot be empty",
		)
	}

	if approval.Signature == "" {
		return fmt.Errorf(
			"reserved transfer proposal approval signature cannot be empty",
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
			"invalid reserved transfer proposal approval signature encoding: %w",
			err,
		)
	}

	if len(signature) !=
		ed25519.SignatureSize {

		return fmt.Errorf(
			"invalid reserved transfer proposal approval signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(proposal.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid reserved transfer proposal approval signature",
		)
	}

	return nil
}
