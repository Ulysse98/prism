package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func pendingAuthorityProposalFixture(
	t *testing.T,
) AuthorityProposal {
	t.Helper()

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	return NewAuthorityProposal(
		"prism-pending-authority-proposal-test",
		1,
		consensus.ReservedPoolTreasury,
		AuthorityChangeAdd,
		target.Address,
	)
}

func TestPendingAuthorityProposalDerivesExecutionHeight(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	pending, err :=
		NewPendingAuthorityProposal(
			proposal,
			10,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	if pending.ProposalHeight != 10 {
		t.Fatalf(
			"unexpected proposal height: got=%d expected=10",
			pending.ProposalHeight,
		)
	}

	if pending.DelayBlocks != 5 {
		t.Fatalf(
			"unexpected delay: got=%d expected=5",
			pending.DelayBlocks,
		)
	}

	if pending.ExecuteAfterHeight != 15 {
		t.Fatalf(
			"unexpected execution height: got=%d expected=15",
			pending.ExecuteAfterHeight,
		)
	}

	if pending.Proposal.ID != proposal.ID {
		t.Fatal(
			"pending proposal changed proposal ID",
		)
	}
}

func TestPendingAuthorityProposalNotExecutableBeforeDelay(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	pending, err :=
		NewPendingAuthorityProposal(
			proposal,
			10,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	if pending.IsExecutableAt(14) {
		t.Fatal(
			"proposal became executable before timelock boundary",
		)
	}

	err =
		pending.RequireExecutableAt(14)

	if err == nil {
		t.Fatal(
			"expected execution before timelock to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"timelock not reached",
	) {
		t.Fatalf(
			"unexpected timelock error: %v",
			err,
		)
	}
}

func TestPendingAuthorityProposalExecutableAtBoundary(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	pending, err :=
		NewPendingAuthorityProposal(
			proposal,
			10,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !pending.IsExecutableAt(15) {
		t.Fatal(
			"proposal was not executable at timelock boundary",
		)
	}

	if err :=
		pending.RequireExecutableAt(
			15,
		); err != nil {

		t.Fatal(err)
	}
}

func TestPendingAuthorityProposalExecutableAfterBoundary(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	pending, err :=
		NewPendingAuthorityProposal(
			proposal,
			10,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !pending.IsExecutableAt(16) {
		t.Fatal(
			"proposal was not executable after timelock boundary",
		)
	}

	if err :=
		pending.RequireExecutableAt(
			16,
		); err != nil {

		t.Fatal(err)
	}
}

func TestPendingAuthorityProposalRejectsZeroDelay(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	if _, err :=
		NewPendingAuthorityProposal(
			proposal,
			10,
			0,
		); err == nil {

		t.Fatal(
			"expected zero governance delay to fail",
		)
	}
}

func TestPendingAuthorityProposalRejectsGenesisHeight(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	if _, err :=
		NewPendingAuthorityProposal(
			proposal,
			0,
			5,
		); err == nil {

		t.Fatal(
			"expected proposal at Genesis height to fail",
		)
	}
}

func TestPendingAuthorityProposalRejectsHeightOverflow(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	if _, err :=
		NewPendingAuthorityProposal(
			proposal,
			^uint64(0)-2,
			5,
		); err == nil {

		t.Fatal(
			"expected execution height overflow to fail",
		)
	}
}

func TestPendingAuthorityProposalRejectsTamperedExecutionHeight(
	t *testing.T,
) {
	proposal :=
		pendingAuthorityProposalFixture(t)

	pending, err :=
		NewPendingAuthorityProposal(
			proposal,
			10,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	pending.ExecuteAfterHeight = 14

	if err :=
		ValidatePendingAuthorityProposal(
			pending,
		); err == nil {

		t.Fatal(
			"expected tampered execution height to fail validation",
		)
	}
}
