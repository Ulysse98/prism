package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func signedReplayAuthorization(
	t *testing.T,
	authorizer *wallet.Wallet,
	pool consensus.ReservedPool,
	nonce uint64,
	amount uint64,
) Authorization {
	t.Helper()

	recipient, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		NewAuthorization(
			testChainID,
			nonce,
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

func TestReplayStateAcceptsFirstAuthorization(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTreasury,
			1,
			100,
		)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	state :=
		NewReplayState()

	if err :=
		state.Accept(
			authorization,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestReplayStateRejectsDuplicateAuthorization(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTreasury,
			1,
			100,
		)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
	}

	state :=
		NewReplayState()

	if err :=
		state.Accept(
			authorization,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.Accept(
			authorization,
			policy,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected duplicate authorization to fail",
		)
	}
}

func TestReplayStateRejectsReusedNonce(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolEcosystem,
			1,
			100,
		)

	second :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolEcosystem,
			1,
			200,
		)

	policy := AuthorityPolicy{
		Ecosystem: []string{
			authorizer.Address,
		},
	}

	state :=
		NewReplayState()

	if err :=
		state.Accept(
			first,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.Accept(
			second,
			policy,
			testChainID,
		); err == nil {

		t.Fatal(
			"expected reused nonce to fail",
		)
	}
}

func TestReplayStateAcceptsIncreasingNonce(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTeam,
			1,
			100,
		)

	second :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTeam,
			2,
			100,
		)

	policy := AuthorityPolicy{
		Team: []string{
			authorizer.Address,
		},
	}

	state :=
		NewReplayState()

	if err :=
		state.Accept(
			first,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.Accept(
			second,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestReplayStateScopesNonceByPool(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	treasury :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTreasury,
			1,
			100,
		)

	team :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolTeam,
			1,
			100,
		)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorizer.Address,
		},
		Team: []string{
			authorizer.Address,
		},
	}

	state :=
		NewReplayState()

	if err :=
		state.Accept(
			treasury,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.Accept(
			team,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(err)
	}
}

func TestReplayStateDoesNotRecordInvalidAuthorization(
	t *testing.T,
) {
	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		signedReplayAuthorization(
			t,
			authorizer,
			consensus.ReservedPoolLiquidity,
			1,
			100,
		)

	policy := AuthorityPolicy{
		Liquidity: []string{
			authorizer.Address,
		},
	}

	state :=
		NewReplayState()

	if err :=
		state.Accept(
			authorization,
			policy,
			"wrong-chain",
		); err == nil {

		t.Fatal(
			"expected wrong-chain authorization to fail",
		)
	}

	if err :=
		state.Accept(
			authorization,
			policy,
			testChainID,
		); err != nil {

		t.Fatal(
			"failed authorization must not consume replay state",
		)
	}
}
