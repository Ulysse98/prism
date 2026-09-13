package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestAddLocalReservedTransferApprovalsMeetsThreshold(
	t *testing.T,
) {
	alice := mustGovernanceWallet(t)
	bob := mustGovernanceWallet(t)
	charlie := mustGovernanceWallet(t)
	recipient := mustGovernanceWallet(t)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			alice.Address,
			bob.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
		)

	wallets := map[string]*wallet.Wallet{
		"Charlie": charlie,
		"Bob":     bob,
		"Alice":   alice,
	}

	approvedBy, err :=
		addLocalReservedTransferApprovals(
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

	if len(proposal.Approvals) != 2 {
		t.Fatalf(
			"expected proposal to contain 2 approvals, got %d",
			len(proposal.Approvals),
		)
	}

	for _, approval := range proposal.Approvals {
		if err :=
			reserved.ValidateReservedTransferProposalApproval(
				proposal,
				approval,
			); err != nil {

			t.Fatalf(
				"invalid reserved transfer approval: %v",
				err,
			)
		}
	}

	if proposal.Approvals[0].Authorizer !=
		alice.Address {

		t.Fatal(
			"first approval should belong to Alice",
		)
	}

	if proposal.Approvals[1].Authorizer !=
		bob.Address {

		t.Fatal(
			"second approval should belong to Bob",
		)
	}
}

func TestAddLocalReservedTransferApprovalsRejectsInsufficientKeys(
	t *testing.T,
) {
	alice := mustGovernanceWallet(t)
	bob := mustGovernanceWallet(t)
	charlie := mustGovernanceWallet(t)
	recipient := mustGovernanceWallet(t)

	policy := reserved.AuthorityPolicy{
		Treasury: []string{
			alice.Address,
			bob.Address,
		},
		TreasuryThreshold: 2,
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
		)

	wallets := map[string]*wallet.Wallet{
		"Alice":   alice,
		"Charlie": charlie,
	}

	approvedBy, err :=
		addLocalReservedTransferApprovals(
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

	// Alice may have signed before discovering that the
	// local threshold cannot be reached.
	if len(proposal.Approvals) != 1 {
		t.Fatalf(
			"expected one partial approval, got %d",
			len(proposal.Approvals),
		)
	}

	if proposal.Approvals[0].Authorizer !=
		alice.Address {

		t.Fatal(
			"unexpected partial approval authorizer",
		)
	}
}

func TestAddLocalReservedTransferApprovalsRejectsNilProposal(
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
		addLocalReservedTransferApprovals(
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
		"reserved transfer proposal cannot be nil",
	) {
		t.Fatalf(
			"unexpected nil proposal error: %v",
			err,
		)
	}
}

func captureReservedTransferCLIOutput(
	t *testing.T,
	run func(),
) string {
	t.Helper()

	original :=
		os.Stdout

	reader, writer, err :=
		os.Pipe()

	if err != nil {
		t.Fatal(err)
	}

	os.Stdout =
		writer

	run()

	if err :=
		writer.Close(); err != nil {

		os.Stdout =
			original

		t.Fatal(err)
	}

	os.Stdout =
		original

	output, err :=
		io.ReadAll(
			reader,
		)

	if closeErr :=
		reader.Close(); closeErr != nil &&
		err == nil {

		err =
			closeErr
	}

	if err != nil {
		t.Fatal(err)
	}

	return string(output)
}

func TestNodeReservedTransferProposeRequiresArguments(
	t *testing.T,
) {
	output :=
		captureReservedTransferCLIOutput(
			t,
			func() {
				runNodeReservedTransferProposeCommand(
					nil,
				)
			},
		)

	if !strings.Contains(
		output,
		"Arguments: <pool> <recipient> <amount> <nonce>",
	) {
		t.Fatalf(
			"unexpected proposal usage output: %q",
			output,
		)
	}
}

func TestNodeReservedTransferProposeRejectsInvalidPool(
	t *testing.T,
) {
	output :=
		captureReservedTransferCLIOutput(
			t,
			func() {
				runNodeReservedTransferProposeCommand(
					[]string{
						"invalid",
						"Bob",
						"100",
						"1",
					},
				)
			},
		)

	if !strings.Contains(
		output,
		"Invalid reserved transfer pool:",
	) {
		t.Fatalf(
			"unexpected invalid-pool output: %q",
			output,
		)
	}
}

func TestNodeReservedTransferProposeRejectsZeroAmount(
	t *testing.T,
) {
	output :=
		captureReservedTransferCLIOutput(
			t,
			func() {
				runNodeReservedTransferProposeCommand(
					[]string{
						"treasury",
						"Bob",
						"0",
						"1",
					},
				)
			},
		)

	if !strings.Contains(
		output,
		"Reserved transfer amount must be greater than zero.",
	) {
		t.Fatalf(
			"unexpected zero-amount output: %q",
			output,
		)
	}
}

func TestNodeReservedTransferProposeRejectsZeroNonce(
	t *testing.T,
) {
	output :=
		captureReservedTransferCLIOutput(
			t,
			func() {
				runNodeReservedTransferProposeCommand(
					[]string{
						"treasury",
						"Bob",
						"100",
						"0",
					},
				)
			},
		)

	if !strings.Contains(
		output,
		"Reserved transfer nonce must be greater than zero.",
	) {
		t.Fatalf(
			"unexpected zero-nonce output: %q",
			output,
		)
	}
}

func TestNodeReservedTransferExecuteRequiresProposalID(
	t *testing.T,
) {
	output :=
		captureReservedTransferCLIOutput(
			t,
			func() {
				runNodeReservedTransferExecuteCommand(
					nil,
				)
			},
		)

	if !strings.Contains(
		output,
		"<proposal-id>",
	) {
		t.Fatalf(
			"unexpected execution usage output: %q",
			output,
		)
	}
}

func TestNodeReservedTransferExecuteRejectsBlankProposalID(
	t *testing.T,
) {
	output :=
		captureReservedTransferCLIOutput(
			t,
			func() {
				runNodeReservedTransferExecuteCommand(
					[]string{
						"   ",
					},
				)
			},
		)

	if !strings.Contains(
		output,
		"Proposal ID cannot be empty.",
	) {
		t.Fatalf(
			"unexpected blank proposal ID output: %q",
			output,
		)
	}
}
