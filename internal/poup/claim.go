package poup

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"prism/internal/wallet"
)

type Claim struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Period    uint64 `json:"period"`
	Points    uint64 `json:"points"`
	Units     uint64 `json:"units"`
	Amount    uint64 `json:"amount"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

func NewClaim(
	address string,
	period uint64,
	points uint64,
	units uint64,
	amount uint64,
	publicKey string,
) Claim {
	claim := Claim{
		Address:   address,
		Period:    period,
		Points:    points,
		Units:     units,
		Amount:    amount,
		PublicKey: publicKey,
	}

	claim.ID = CalculateID(claim)

	return claim
}

func claimPayload(
	claim Claim,
) string {
	return fmt.Sprintf(
		"%s|%d|%d|%d|%d|%s",
		claim.Address,
		claim.Period,
		claim.Points,
		claim.Units,
		claim.Amount,
		claim.PublicKey,
	)
}

func CalculateID(
	claim Claim,
) string {
	hash := sha256.Sum256(
		[]byte(claimPayload(claim)),
	)

	return hex.EncodeToString(hash[:])
}

func (claim *Claim) Sign(
	privateKey ed25519.PrivateKey,
) error {
	if claim == nil {
		return fmt.Errorf(
			"participation claim cannot be nil",
		)
	}

	publicKey, err := wallet.DecodePublicKey(
		claim.PublicKey,
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
			"private key does not match claim public key",
		)
	}

	expectedAddress :=
		wallet.AddressFromPublicKey(
			publicKey,
		)

	if claim.Address != expectedAddress {
		return fmt.Errorf(
			"claim public key does not own address",
		)
	}

	claim.ID = CalculateID(*claim)

	signature := ed25519.Sign(
		privateKey,
		[]byte(claim.ID),
	)

	claim.Signature =
		hex.EncodeToString(signature)

	return nil
}

func ValidateSigned(
	claim Claim,
) error {
	if claim.Address == "" {
		return fmt.Errorf(
			"participation claim address cannot be empty",
		)
	}

	if claim.Address == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot claim participation rewards",
		)
	}

	if claim.Points == 0 {
		return fmt.Errorf(
			"participation claim points must be greater than zero",
		)
	}

	if claim.Units == 0 {
		return fmt.Errorf(
			"participation claim units must be greater than zero",
		)
	}

	if claim.Amount == 0 {
		return fmt.Errorf(
			"participation claim amount must be greater than zero",
		)
	}

	if claim.PublicKey == "" {
		return fmt.Errorf(
			"participation claim public key cannot be empty",
		)
	}

	if claim.Signature == "" {
		return fmt.Errorf(
			"participation claim signature cannot be empty",
		)
	}

	if claim.ID != CalculateID(claim) {
		return fmt.Errorf(
			"invalid participation claim ID",
		)
	}

	publicKey, err := wallet.DecodePublicKey(
		claim.PublicKey,
	)
	if err != nil {
		return err
	}

	expectedAddress :=
		wallet.AddressFromPublicKey(
			publicKey,
		)

	if claim.Address != expectedAddress {
		return fmt.Errorf(
			"participation claim public key does not own address",
		)
	}

	signature, err :=
		hex.DecodeString(
			claim.Signature,
		)

	if err != nil {
		return fmt.Errorf(
			"invalid participation claim signature encoding: %w",
			err,
		)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf(
			"invalid participation claim signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(claim.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid participation claim signature",
		)
	}

	return nil
}
