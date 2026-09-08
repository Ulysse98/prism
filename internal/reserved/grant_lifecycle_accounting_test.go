package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func accountingLifecycleGrant(
	t *testing.T,
	nonce uint64,
	notBefore uint64,
	expiresAt uint64,
) (
	Grant,
	AuthorityPolicy,
	[]*wallet.Wallet,
) {
	t.Helper()

	base, policy, authorities :=
		accountingThresholdGrant(
			t,
			nonce,
		)

	grant :=
		NewGrantWithWindow(
			testChainID,
			nonce,
			consensus.ReservedPoolTreasury,
			base.Recipient,
			base.Amount,
			notBefore,
			expiresAt,
		)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&grant,
		authorities[1],
	)

	return grant,
		policy,
		authorities
}

func TestAccountingStateRejectsGrantBeforeActivationWithoutConsumingNonce(
	t *testing.T,
) {
	grant, policy, _ :=
		accountingLifecycleGrant(
			t,
			1,
			5,
			10,
		)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptGrantAtHeight(
			grant,
			4,
			policy,
			testChainID,
			consensus.DefaultSupplyPolicy(),
		); err == nil {

		t.Fatal(
			"expected grant before activation to fail",
		)
	}

	if state.Usage.Treasury != 0 {
		t.Fatal(
			"premature grant changed reserved usage",
		)
	}

	if err :=
		state.AcceptGrantAtHeight(
			grant,
			5,
			policy,
			testChainID,
			consensus.DefaultSupplyPolicy(),
		); err != nil {

		t.Fatal(
			"premature attempt consumed grant nonce:",
			err,
		)
	}

	if state.Usage.Treasury != grant.Amount {
		t.Fatalf(
			"expected treasury usage %d, got %d",
			grant.Amount,
			state.Usage.Treasury,
		)
	}
}

func TestAccountingStateAcceptsGrantAtExpirationBoundary(
	t *testing.T,
) {
	grant, policy, _ :=
		accountingLifecycleGrant(
			t,
			1,
			5,
			10,
		)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptGrantAtHeight(
			grant,
			10,
			policy,
			testChainID,
			consensus.DefaultSupplyPolicy(),
		); err != nil {

		t.Fatal(err)
	}
}

func TestAccountingStateRejectsExpiredGrant(
	t *testing.T,
) {
	grant, policy, _ :=
		accountingLifecycleGrant(
			t,
			1,
			5,
			10,
		)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptGrantAtHeight(
			grant,
			11,
			policy,
			testChainID,
			consensus.DefaultSupplyPolicy(),
		); err == nil {

		t.Fatal(
			"expected expired grant to fail",
		)
	}

	if state.Usage.Treasury != 0 {
		t.Fatal(
			"expired grant changed reserved usage",
		)
	}
}

func TestAccountingStateRequiresExplicitHeightForLifecycleGrant(
	t *testing.T,
) {
	grant, policy, _ :=
		accountingLifecycleGrant(
			t,
			1,
			0,
			10,
		)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptGrant(
			grant,
			policy,
			testChainID,
			consensus.DefaultSupplyPolicy(),
		); err == nil {

		t.Fatal(
			"expected lifecycle grant without explicit height to fail",
		)
	}

	if state.Usage.Treasury != 0 {
		t.Fatal(
			"heightless lifecycle grant changed reserved usage",
		)
	}

	if err :=
		state.AcceptGrantAtHeight(
			grant,
			1,
			policy,
			testChainID,
			consensus.DefaultSupplyPolicy(),
		); err != nil {

		t.Fatal(
			"heightless attempt consumed lifecycle grant:",
			err,
		)
	}
}
