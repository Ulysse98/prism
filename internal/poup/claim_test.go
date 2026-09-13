package poup

import (
	"testing"

	"prism/internal/wallet"
)

func TestSignedClaimValidates(
	t *testing.T,
) {
	actor, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claim := NewClaim(
		actor.Address,
		0,
		1200,
		10,
		10,
		actor.PublicKeyHex(),
	)

	if err := claim.Sign(
		actor.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if err := ValidateSigned(
		claim,
	); err != nil {
		t.Fatal(err)
	}
}

func TestClaimTamperingFails(
	t *testing.T,
) {
	actor, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claim := NewClaim(
		actor.Address,
		0,
		1200,
		10,
		10,
		actor.PublicKeyHex(),
	)

	if err := claim.Sign(
		actor.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	claim.Amount = 11

	if err := ValidateSigned(
		claim,
	); err == nil {
		t.Fatal(
			"expected tampered claim to fail",
		)
	}
}

func TestClaimRejectsWrongSigner(
	t *testing.T,
) {
	actor, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	other, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	claim := NewClaim(
		actor.Address,
		0,
		1200,
		10,
		10,
		actor.PublicKeyHex(),
	)

	if err := claim.Sign(
		other.PrivateKey,
	); err == nil {
		t.Fatal(
			"expected wrong signer to fail",
		)
	}
}
