package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestReservedTransferProposalAddApproval(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
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

	if len(proposal.Approvals) != 1 {
		t.Fatalf(
			"expected one approval, got %d",
			len(proposal.Approvals),
		)
	}

	if err :=
		ValidateReservedTransferProposalApproval(
			proposal,
			proposal.Approvals[0],
		); err != nil {

		t.Fatalf(
			"approval validation failed: %v",
			err,
		)
	}
}

func TestReservedTransferProposalApprovalDoesNotChangeID(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	before := proposal.ID

	if err :=
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	if proposal.ID != before {
		t.Fatal(
			"adding approval changed proposal ID",
		)
	}
}

func TestReservedTransferProposalRejectsDuplicateApproval(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
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

	err =
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		)

	if err == nil {
		t.Fatal(
			"expected duplicate approval to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already approved",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReservedTransferProposalRejectsWrongPrivateKey(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	other, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	err =
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			other.PrivateKey,
		)

	if err == nil {
		t.Fatal(
			"expected wrong private key to be rejected",
		)
	}
}

func TestReservedTransferProposalRejectsWrongAuthorizer(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	other, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	err =
		proposal.AddApproval(
			other.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		)

	if err == nil {
		t.Fatal(
			"expected wrong authorizer to be rejected",
		)
	}
}

func TestReservedTransferProposalRejectsTamperedApproval(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
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

	proposal.Approvals[0].Signature =
		"00" +
			proposal.Approvals[0].Signature[2:]

	err =
		ValidateReservedTransferProposalApproval(
			proposal,
			proposal.Approvals[0],
		)

	if err == nil {
		t.Fatal(
			"expected tampered approval to be rejected",
		)
	}
}
