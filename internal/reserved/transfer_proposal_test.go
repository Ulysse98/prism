package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
)

func TestNewReservedTransferProposal(t *testing.T) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient_001",
			250,
		)

	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err != nil {

		t.Fatal(err)
	}

	if proposal.ChainID != "prism-devnet" {
		t.Fatalf(
			"unexpected chain ID: %q",
			proposal.ChainID,
		)
	}

	if proposal.Nonce != 1 {
		t.Fatalf(
			"unexpected nonce: %d",
			proposal.Nonce,
		)
	}

	if proposal.Pool !=
		consensus.ReservedPoolTreasury {

		t.Fatalf(
			"unexpected pool: %q",
			proposal.Pool,
		)
	}

	if proposal.Recipient != "recipient_001" {
		t.Fatalf(
			"unexpected recipient: %q",
			proposal.Recipient,
		)
	}

	if proposal.Amount != 250 {
		t.Fatalf(
			"unexpected amount: %d",
			proposal.Amount,
		)
	}

	if len(proposal.Approvals) != 0 {
		t.Fatalf(
			"expected no approvals, got %d",
			len(proposal.Approvals),
		)
	}

	if proposal.ID == "" {
		t.Fatal(
			"expected proposal ID",
		)
	}
}

func TestReservedTransferProposalIDCommitsIntent(
	t *testing.T,
) {
	base :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient_001",
			250,
		)

	changedAmount := base
	changedAmount.Amount = 251

	if CalculateReservedTransferProposalID(
		base,
	) ==
		CalculateReservedTransferProposalID(
			changedAmount,
		) {

		t.Fatal(
			"proposal ID does not commit amount",
		)
	}

	changedRecipient := base
	changedRecipient.Recipient =
		"recipient_002"

	if CalculateReservedTransferProposalID(
		base,
	) ==
		CalculateReservedTransferProposalID(
			changedRecipient,
		) {

		t.Fatal(
			"proposal ID does not commit recipient",
		)
	}
}

func TestValidateReservedTransferProposalRejectsEmptyChainID(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	err :=
		ValidateReservedTransferProposal(
			proposal,
		)

	if err == nil {
		t.Fatal(
			"expected empty chain ID to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"chain ID cannot be empty",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestValidateReservedTransferProposalRejectsZeroNonce(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			0,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected zero nonce to be rejected",
		)
	}
}

func TestValidateReservedTransferProposalRejectsEmptyRecipient(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"",
			100,
		)

	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected empty recipient to be rejected",
		)
	}
}

func TestValidateReservedTransferProposalRejectsGenesisRecipient(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"GENESIS",
			100,
		)

	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected GENESIS recipient to be rejected",
		)
	}
}

func TestValidateReservedTransferProposalRejectsZeroAmount(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			0,
		)

	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected zero amount to be rejected",
		)
	}
}

func TestValidateReservedTransferProposalRejectsInvalidPool(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPool("invalid"),
			"recipient",
			100,
		)

	if err :=
		ValidateReservedTransferProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected invalid pool to be rejected",
		)
	}
}

func TestValidateReservedTransferProposalRejectsTamperedID(
	t *testing.T,
) {
	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	proposal.Amount = 101

	err :=
		ValidateReservedTransferProposal(
			proposal,
		)

	if err == nil {
		t.Fatal(
			"expected tampered proposal to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid reserved transfer proposal ID",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
