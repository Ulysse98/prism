package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

const testChainID = "prism-test-chain"

func newSignedAuthorization(
	t *testing.T,
	pool consensus.ReservedPool,
	amount uint64,
) Authorization {
	t.Helper()

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
			testChainID,
			1,
			pool,
			recipient.Address,
			amount,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	return authorization
}

func TestSignedAuthorizationValidates(
	t *testing.T,
) {
	authorization :=
		newSignedAuthorization(
			t,
			consensus.ReservedPoolTreasury,
			100,
		)

	if err :=
		ValidateSignedForChain(
			authorization,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorizationRejectsTampering(
	t *testing.T,
) {
	authorization :=
		newSignedAuthorization(
			t,
			consensus.ReservedPoolEcosystem,
			100,
		)

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
	authorization :=
		newSignedAuthorization(
			t,
			consensus.ReservedPool("unknown"),
			100,
		)

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
	authorization :=
		newSignedAuthorization(
			t,
			consensus.ReservedPoolLiquidity,
			5_000_001,
		)

	if err :=
		ValidateSigned(
			authorization,
		); err == nil {

		t.Fatal(
			"expected pool allocation overflow to fail",
		)
	}
}

func TestAuthorizationRejectsWrongChain(
	t *testing.T,
) {
	authorization :=
		newSignedAuthorization(
			t,
			consensus.ReservedPoolTreasury,
			100,
		)

	if err :=
		ValidateSignedForChain(
			authorization,
			"other-chain",
		); err == nil {

		t.Fatal(
			"expected chain ID mismatch to fail",
		)
	}
}

func TestAuthorizationNonceIsSigned(
	t *testing.T,
) {
	authorization :=
		newSignedAuthorization(
			t,
			consensus.ReservedPoolTeam,
			100,
		)

	authorization.Nonce++

	if err :=
		ValidateSigned(
			authorization,
		); err == nil {

		t.Fatal(
			"expected nonce tampering to fail",
		)
	}
}
