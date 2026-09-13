package main

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func mustGovernanceWallet(
	t *testing.T,
) *wallet.Wallet {
	t.Helper()

	currentWallet, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	return currentWallet
}

func TestParseGovernancePool(t *testing.T) {
	tests := []struct {
		input    string
		expected consensus.ReservedPool
	}{
		{
			input:    "ecosystem",
			expected: consensus.ReservedPoolEcosystem,
		},
		{
			input:    "TREASURY",
			expected: consensus.ReservedPoolTreasury,
		},
		{
			input:    " team ",
			expected: consensus.ReservedPoolTeam,
		},
		{
			input:    "Liquidity",
			expected: consensus.ReservedPoolLiquidity,
		},
	}

	for _, test := range tests {
		actual, err :=
			parseGovernancePool(
				test.input,
			)

		if err != nil {
			t.Fatalf(
				"parseGovernancePool(%q): %v",
				test.input,
				err,
			)
		}

		if actual != test.expected {
			t.Fatalf(
				"parseGovernancePool(%q): expected=%q got=%q",
				test.input,
				test.expected,
				actual,
			)
		}
	}
}

func TestParseGovernancePoolRejectsInvalid(t *testing.T) {
	_, err :=
		parseGovernancePool(
			"unknown",
		)

	if err == nil {
		t.Fatal(
			"expected invalid governance pool to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"expected ecosystem, treasury, team, or liquidity",
	) {
		t.Fatalf(
			"unexpected governance pool error: %v",
			err,
		)
	}
}

func TestParseGovernanceAction(t *testing.T) {
	tests := []struct {
		input    string
		expected reserved.AuthorityChangeAction
	}{
		{
			input:    "add",
			expected: reserved.AuthorityChangeAdd,
		},
		{
			input:    " REMOVE ",
			expected: reserved.AuthorityChangeRemove,
		},
	}

	for _, test := range tests {
		actual, err :=
			parseGovernanceAction(
				test.input,
			)

		if err != nil {
			t.Fatalf(
				"parseGovernanceAction(%q): %v",
				test.input,
				err,
			)
		}

		if actual != test.expected {
			t.Fatalf(
				"parseGovernanceAction(%q): expected=%q got=%q",
				test.input,
				test.expected,
				actual,
			)
		}
	}
}

func TestParseGovernanceActionRejectsInvalid(t *testing.T) {
	_, err :=
		parseGovernanceAction(
			"rotate",
		)

	if err == nil {
		t.Fatal(
			"expected invalid governance action to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"expected add or remove",
	) {
		t.Fatalf(
			"unexpected governance action error: %v",
			err,
		)
	}
}

func TestAddLocalGovernanceApprovalsMeetsThreshold(
	t *testing.T,
) {
	alice := mustGovernanceWallet(t)
	bob := mustGovernanceWallet(t)
	charlie := mustGovernanceWallet(t)
	target := mustGovernanceWallet(t)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			alice.Address,
			bob.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		reserved.NewAuthorityProposal(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target.Address,
		)

	wallets := map[string]*wallet.Wallet{
		"Charlie": charlie,
		"Bob":     bob,
		"Alice":   alice,
	}

	approvedBy, err :=
		addLocalGovernanceApprovals(
			&proposal,
			policy,
			consensus.ReservedPoolTreasury,
			2,
			wallets,
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(approvedBy) != 2 {
		t.Fatalf(
			"expected 2 local approvals, got %d",
			len(approvedBy),
		)
	}

	// Wallet names are sorted before approval selection.
	if approvedBy[0] != "Alice" ||
		approvedBy[1] != "Bob" {

		t.Fatalf(
			"unexpected approval order: %v",
			approvedBy,
		)
	}

	if len(proposal.Change.Approvals) != 2 {
		t.Fatalf(
			"expected proposal to contain 2 approvals, got %d",
			len(proposal.Change.Approvals),
		)
	}

	for _, approval := range proposal.Change.Approvals {

		if err :=
			reserved.ValidateAuthorityProposalApproval(
				proposal,
				approval,
			); err != nil {

			t.Fatalf(
				"invalid proposal approval: %v",
				err,
			)
		}
	}

	if proposal.Change.Approvals[0].Authorizer !=
		alice.Address {

		t.Fatalf(
			"first approval should belong to Alice",
		)
	}

	if proposal.Change.Approvals[1].Authorizer !=
		bob.Address {

		t.Fatalf(
			"second approval should belong to Bob",
		)
	}
}

func TestAddLocalGovernanceApprovalsRejectsInsufficientKeys(
	t *testing.T,
) {
	alice := mustGovernanceWallet(t)
	bob := mustGovernanceWallet(t)
	charlie := mustGovernanceWallet(t)
	target := mustGovernanceWallet(t)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			alice.Address,
			bob.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		reserved.NewAuthorityProposal(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target.Address,
		)

	wallets := map[string]*wallet.Wallet{
		"Alice":   alice,
		"Charlie": charlie,
	}

	approvedBy, err :=
		addLocalGovernanceApprovals(
			&proposal,
			policy,
			consensus.ReservedPoolTreasury,
			2,
			wallets,
		)

	if err == nil {
		t.Fatal(
			"expected insufficient authority keys to be rejected",
		)
	}

	if approvedBy != nil {
		t.Fatalf(
			"expected nil approval result on failure, got %v",
			approvedBy,
		)
	}

	if !strings.Contains(
		err.Error(),
		"not enough local authority keys",
	) {
		t.Fatalf(
			"unexpected insufficient-key error: %v",
			err,
		)
	}

	// Alice's valid partial signature may have been added before
	// discovering that the local threshold cannot be reached.
	if len(proposal.Change.Approvals) != 1 {
		t.Fatalf(
			"expected one partial approval, got %d",
			len(proposal.Change.Approvals),
		)
	}

	if proposal.Change.Approvals[0].Authorizer !=
		alice.Address {

		t.Fatalf(
			"unexpected partial approval authorizer",
		)
	}
}

func TestAddLocalGovernanceApprovalsRejectsNilProposal(
	t *testing.T,
) {
	alice := mustGovernanceWallet(t)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			alice.Address,
		},
		TreasuryThreshold: 1,
	}

	_, err :=
		addLocalGovernanceApprovals(
			nil,
			policy,
			consensus.ReservedPoolTreasury,
			1,
			map[string]*wallet.Wallet{
				"Alice": alice,
			},
		)

	if err == nil {
		t.Fatal(
			"expected nil proposal to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"authority proposal cannot be nil",
	) {
		t.Fatalf(
			"unexpected nil proposal error: %v",
			err,
		)
	}
}
