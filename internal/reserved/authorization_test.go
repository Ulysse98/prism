package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestSignedAuthorizationValidates(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	recipient, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		NewAuthorization(
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		ValidateSigned(
			authorization,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorizationRejectsTampering(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	recipient, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		NewAuthorization(
			consensus.ReservedPoolEcosystem,
			recipient.Address,
			100,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	authorization.Amount = 101

	if err :=
		ValidateSigned(
			authorization,
		); err == nil {

		t.Fatal(
			"expected tampered authorization to fail",
		)
	}
}

func TestAuthorizationRejectsUnknownPool(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		NewAuthorization(
			consensus.ReservedPool("unknown"),
			"recipient",
			100,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		ValidateSigned(
			authorization,
		); err == nil {

		t.Fatal(
			"expected unknown reserved pool to fail",
		)
	}
}

func TestAuthorizationRejectsPoolAllocationOverflow(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		NewAuthorization(
			consensus.ReservedPoolLiquidity,
			"recipient",
			5_000_001,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		ValidateSigned(
			authorization,
		); err == nil {

		t.Fatal(
			"expected pool allocation overflow to fail",
		)
	}
}
