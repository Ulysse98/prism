package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func mustReservedTransferWallet(
	t *testing.T,
) *wallet.Wallet {
	t.Helper()

	currentWallet, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	return currentWallet
}

func TestReservedTransferPolicyAcceptsQuorum(
	t *testing.T,
) {
	authorityA :=
		mustReservedTransferWallet(t)

	authorityB :=
		mustReservedTransferWallet(t)

	authorityC :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
			authorityC.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	for _, authority := range []*wallet.Wallet{
		authorityA,
		authorityB,
	} {

		if err :=
			proposal.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	if err :=
		policy.ValidateReservedTransferProposal(
			proposal,
			"prism-devnet",
		); err != nil {

		t.Fatalf(
			"valid transfer quorum rejected: %v",
			err,
		)
	}
}

func TestReservedTransferPolicyRejectsMissingQuorum(
	t *testing.T,
) {
	authorityA :=
		mustReservedTransferWallet(t)

	authorityB :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		proposal.AddApproval(
			authorityA.Address,
			authorityA.PublicKeyHex(),
			authorityA.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	err :=
		policy.ValidateReservedTransferProposal(
			proposal,
			"prism-devnet",
		)

	if err == nil {
		t.Fatal(
			"expected missing quorum to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"approval threshold not met",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReservedTransferPolicyRejectsUnauthorizedApprover(
	t *testing.T,
) {
	authorityA :=
		mustReservedTransferWallet(t)

	authorityB :=
		mustReservedTransferWallet(t)

	outsider :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	for _, authority := range []*wallet.Wallet{
		authorityA,
		outsider,
	} {

		if err :=
			proposal.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	err :=
		policy.ValidateReservedTransferProposal(
			proposal,
			"prism-devnet",
		)

	if err == nil {
		t.Fatal(
			"expected unauthorized approver to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"approver is not authorized",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReservedTransferPolicyRejectsWrongChain(
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
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	err :=
		policy.ValidateReservedTransferProposal(
			proposal,
			"another-chain",
		)

	if err == nil {
		t.Fatal(
			"expected wrong chain to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"chain ID mismatch",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReservedTransferPolicyRejectsDuplicateApprover(
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
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	proposal.Approvals =
		append(
			proposal.Approvals,
			proposal.Approvals[0],
		)

	err :=
		policy.ValidateReservedTransferProposal(
			proposal,
			"prism-devnet",
		)

	if err == nil {
		t.Fatal(
			"expected duplicate approver to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"duplicate reserved transfer proposal approver",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReservedTransferPolicyZeroThresholdDefaultsToOne(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	policy := AuthorityPolicy{
		Treasury: []string{
			authority.Address,
		},
		TreasuryThreshold: 0,
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		policy.ValidateReservedTransferProposal(
			proposal,
			"prism-devnet",
		); err != nil {

		t.Fatalf(
			"legacy single-authority threshold rejected: %v",
			err,
		)
	}
}
