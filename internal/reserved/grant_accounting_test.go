package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func accountingThresholdGrant(
	t *testing.T,
	nonce uint64,
) (
	Grant,
	AuthorityPolicy,
	[]*wallet.Wallet,
) {
	t.Helper()

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorities := make([]*wallet.Wallet, 3)
	addresses := make([]string, 3)

	for i := range authorities {
		authorities[i], err = wallet.New()
		if err != nil {
			t.Fatal(err)
		}

		addresses[i] = authorities[i].Address
	}

	grant := NewGrant(
		testChainID,
		nonce,
		consensus.ReservedPoolTreasury,
		recipient.Address,
		100,
	)

	policy := AuthorityPolicy{
		Treasury:          addresses,
		TreasuryThreshold: 2,
	}

	return grant, policy, authorities
}

func TestAccountingStateAcceptsThresholdGrantOnce(
	t *testing.T,
) {
	grant, policy, authorities :=
		accountingThresholdGrant(t, 1)

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

	state := NewAccountingState(0)

	if err := state.AcceptGrant(
		grant,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err != nil {
		t.Fatal(err)
	}

	if state.Usage.Treasury != 100 {
		t.Fatalf(
			"expected treasury usage 100, got %d",
			state.Usage.Treasury,
		)
	}

	if err := state.AcceptGrant(
		grant,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err == nil {
		t.Fatal(
			"expected duplicate grant to fail",
		)
	}

	if state.Usage.Treasury != 100 {
		t.Fatalf(
			"duplicate grant changed treasury usage: %d",
			state.Usage.Treasury,
		)
	}
}

func TestAccountingStateThresholdFailureDoesNotConsumeGrantNonce(
	t *testing.T,
) {
	grant, policy, authorities :=
		accountingThresholdGrant(t, 1)

	approveThresholdGrant(
		t,
		&grant,
		authorities[0],
	)

	state := NewAccountingState(0)

	if err := state.AcceptGrant(
		grant,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err == nil {
		t.Fatal(
			"expected below-threshold grant to fail",
		)
	}

	if state.Usage.Treasury != 0 {
		t.Fatal(
			"failed grant changed reserved usage",
		)
	}

	approveThresholdGrant(
		t,
		&grant,
		authorities[1],
	)

	if err := state.AcceptGrant(
		grant,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err != nil {
		t.Fatal(
			"failed threshold attempt consumed grant nonce:",
			err,
		)
	}

	if state.Usage.Treasury != 100 {
		t.Fatalf(
			"expected treasury usage 100, got %d",
			state.Usage.Treasury,
		)
	}
}

func TestAccountingStateRejectsNonIncreasingGrantNonce(
	t *testing.T,
) {
	first, policy, authorities :=
		accountingThresholdGrant(t, 2)

	approveThresholdGrant(
		t,
		&first,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&first,
		authorities[1],
	)

	state := NewAccountingState(0)

	if err := state.AcceptGrant(
		first,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	second := NewGrant(
		testChainID,
		1,
		consensus.ReservedPoolTreasury,
		recipient.Address,
		100,
	)

	approveThresholdGrant(
		t,
		&second,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&second,
		authorities[1],
	)

	if err := state.AcceptGrant(
		second,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err == nil {
		t.Fatal(
			"expected non-increasing grant nonce to fail",
		)
	}
}

func TestAccountingStateScopesGrantNonceByPool(
	t *testing.T,
) {
	treasury, policy, authorities :=
		accountingThresholdGrant(t, 1)

	approveThresholdGrant(
		t,
		&treasury,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&treasury,
		authorities[1],
	)

	policy.Ecosystem = []string{
		authorities[0].Address,
		authorities[1].Address,
		authorities[2].Address,
	}
	policy.EcosystemThreshold = 2

	state := NewAccountingState(0)

	if err := state.AcceptGrant(
		treasury,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	ecosystem := NewGrant(
		testChainID,
		1,
		consensus.ReservedPoolEcosystem,
		recipient.Address,
		100,
	)

	approveThresholdGrant(
		t,
		&ecosystem,
		authorities[0],
	)

	approveThresholdGrant(
		t,
		&ecosystem,
		authorities[1],
	)

	if err := state.AcceptGrant(
		ecosystem,
		policy,
		testChainID,
		consensus.DefaultSupplyPolicy(),
	); err != nil {
		t.Fatal(
			"grant nonce should be scoped by pool:",
			err,
		)
	}
}
