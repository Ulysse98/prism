package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
)

func TestAccountingStateAcceptsReservedTransfer(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
		TreasuryThreshold: 1,
	}

	proposal :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptReservedTransfer(
			proposal,
			policy,
			"prism-devnet",
			consensus.DefaultSupplyPolicy(),
		); err != nil {

		t.Fatal(err)
	}

	if state.Usage.Treasury != 100 {
		t.Fatalf(
			"unexpected treasury usage: %d",
			state.Usage.Treasury,
		)
	}
}

func TestAccountingStateRejectsDuplicateReservedTransfer(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
		TreasuryThreshold: 1,
	}

	proposal :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	state :=
		NewAccountingState(0)

	if err :=
		state.AcceptReservedTransfer(
			proposal,
			policy,
			"prism-devnet",
			consensus.DefaultSupplyPolicy(),
		); err != nil {

		t.Fatal(err)
	}

	err :=
		state.AcceptReservedTransfer(
			proposal,
			policy,
			"prism-devnet",
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected duplicate execution to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already executed",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestAccountingStateRejectsNonIncreasingReservedTransferNonce(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
		TreasuryThreshold: 1,
	}

	state :=
		NewAccountingState(0)

	second :=
		signedTreasuryTransfer(
			t,
			authority,
			2,
		)

	if err :=
		state.AcceptReservedTransfer(
			second,
			policy,
			"prism-devnet",
			consensus.DefaultSupplyPolicy(),
		); err != nil {

		t.Fatal(err)
	}

	first :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	err :=
		state.AcceptReservedTransfer(
			first,
			policy,
			"prism-devnet",
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected stale transfer nonce to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"nonce is not increasing",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestAccountingStateRejectsReservedTransferOverRemainingBudget(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
		TreasuryThreshold: 1,
	}

	state :=
		NewAccountingState(0)

	state.Usage.Treasury =
		consensus.TreasuryAllocation

	proposal :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	err :=
		state.AcceptReservedTransfer(
			proposal,
			policy,
			"prism-devnet",
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected exhausted treasury budget to reject transfer",
		)
	}

	if !strings.Contains(
		err.Error(),
		"reserved budget rejected transfer",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
