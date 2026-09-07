package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func signedReservedAuthorizationForBlockchain(
	t *testing.T,
	bc *Blockchain,
	authorizer *wallet.Wallet,
) reserved.Authorization {
	t.Helper()

	chainID, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	recipient, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		reserved.NewAuthorization(
			chainID,
			1,
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

	return authorization
}

func TestBlockchainValidatesReservedAuthorizationForOwnChain(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReservedAuthorizationForBlockchain(
			t,
			bc,
			authorizer,
		)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	if err :=
		bc.ValidateReservedAuthorization(
			authorization,
			policy,
		); err != nil {

		t.Fatal(err)
	}
}

func TestBlockchainRejectsReservedAuthorizationForWrongChain(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReservedAuthorizationForBlockchain(
			t,
			bc,
			authorizer,
		)

	authorization.ChainID =
		"prism-wrong-chain"

	authorization.ID =
		reserved.CalculateID(
			authorization,
		)

	if err :=
		authorization.Sign(
			authorizer.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	if err :=
		bc.ValidateReservedAuthorization(
			authorization,
			policy,
		); err == nil {

		t.Fatal(
			"expected wrong-chain reserved authorization to fail",
		)
	}
}

func TestBlockchainDefaultAuthorityPolicyStillDenies(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReservedAuthorizationForBlockchain(
			t,
			bc,
			authorizer,
		)

	if err :=
		bc.ValidateReservedAuthorization(
			authorization,
			reserved.DefaultAuthorityPolicy(),
		); err == nil {

		t.Fatal(
			"expected default authority policy to deny authorization",
		)
	}
}
