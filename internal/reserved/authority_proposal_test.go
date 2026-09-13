package reserved

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestAuthorityProposalIDDeterministic(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	const chainID = "prism-authority-proposal-test"

	first :=
		NewAuthorityProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	second :=
		NewAuthorityProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if first.ID != second.ID {
		t.Fatal(
			"identical authority proposals produced different IDs",
		)
	}

	if first.Change.ID != second.Change.ID {
		t.Fatal(
			"identical proposal changes produced different IDs",
		)
	}
}

func TestAuthorityProposalIDDoesNotDependOnApprovals(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewAuthorityProposal(
			"prism-authority-proposal-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	before :=
		proposal.ID

	if err := proposal.AddApproval(
		authority.Address,
		authority.PublicKeyHex(),
		authority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if proposal.ID != before {
		t.Fatal(
			"adding approval changed authority proposal ID",
		)
	}

	if len(proposal.Change.Approvals) != 1 {
		t.Fatalf(
			"expected one proposal approval, got %d",
			len(proposal.Change.Approvals),
		)
	}
}

func TestAuthorityProposalApprovalValidates(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewAuthorityProposal(
			"prism-authority-proposal-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	if err := proposal.AddApproval(
		authority.Address,
		authority.PublicKeyHex(),
		authority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	if err :=
		ValidateAuthorityProposalApproval(
			proposal,
			proposal.Change.Approvals[0],
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityProposalRejectsTamperedChange(
	t *testing.T,
) {
	firstTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	secondTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewAuthorityProposal(
			"prism-authority-proposal-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			firstTarget.Address,
		)

	proposal.Change.Authority =
		secondTarget.Address

	if err :=
		ValidateAuthorityProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected tampered authority proposal to fail validation",
		)
	}
}

func TestAuthorityProposalRejectsDirectActivationHeight(
	t *testing.T,
) {
	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		NewAuthorityProposal(
			"prism-authority-proposal-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target.Address,
		)

	proposal.Change.ActivationHeight = 10
	proposal.Change.ID =
		CalculateAuthorityChangeID(
			proposal.Change,
		)

	proposal.ID =
		CalculateAuthorityProposalID(
			proposal,
		)

	if err :=
		ValidateAuthorityProposal(
			proposal,
		); err == nil {

		t.Fatal(
			"expected queued proposal with direct activation height to fail",
		)
	}
}

func TestAuthorityProposalChangeIDIsBoundIntoProposalID(
	t *testing.T,
) {
	firstTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	secondTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	first :=
		NewAuthorityProposal(
			"prism-authority-proposal-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			firstTarget.Address,
		)

	second :=
		NewAuthorityProposal(
			"prism-authority-proposal-test",
			1,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			secondTarget.Address,
		)

	if first.Change.ID == second.Change.ID {
		t.Fatal(
			"different authority changes produced identical IDs",
		)
	}

	if first.ID == second.ID {
		t.Fatal(
			"different authority changes produced identical proposal IDs",
		)
	}
}
