package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func signedAuthorizationForAuthorityTest(
	t *testing.T,
	pool consensus.ReservedPool,
) (
	Authorization,
	*wallet.Wallet,
) {
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

	return authorization,
		authorizer
}

func TestDefaultAuthorityPolicyDeniesAuthorization(
	t *testing.T,
) {
	authorization, _ :=
		signedAuthorizationForAuthorityTest(
			t,
			consensus.ReservedPoolTreasury,
		)

	policy :=
		DefaultAuthorityPolicy()

	if err :=
		policy.ValidateAuthorization(
			authorization,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected default authority policy to deny authorization",
		)
	}
}

func TestAuthorityPolicyAllowsConfiguredAuthorizer(
	t *testing.T,
) {
	authorization, authorizer :=
		signedAuthorizationForAuthorityTest(
			t,
			consensus.ReservedPoolTreasury,
		)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	if err :=
		policy.ValidateAuthorization(
			authorization,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityPolicyRejectsAuthorizerForWrongPool(
	t *testing.T,
) {
	authorization, authorizer :=
		signedAuthorizationForAuthorityTest(
			t,
			consensus.ReservedPoolTeam,
		)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	if err :=
		policy.ValidateAuthorization(
			authorization,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected authorizer configured for another pool to fail",
		)
	}
}

func TestAuthorityPolicyStillRejectsInvalidSignature(
	t *testing.T,
) {
	authorization, authorizer :=
		signedAuthorizationForAuthorityTest(
			t,
			consensus.ReservedPoolEcosystem,
		)

	policy := AuthorityPolicy{
		Ecosystem: []string{
			authorizer.Address,
		},
	}

	authorization.Amount++

	if err :=
		policy.ValidateAuthorization(
			authorization,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected invalid signature to fail before authority approval",
		)
	}
}
