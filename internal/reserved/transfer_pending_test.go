package reserved

import (
	"math"
	"strings"
	"testing"

	"prism/internal/consensus"
)

func testReservedTransferProposal() ReservedTransferProposal {
	return NewReservedTransferProposal(
		"prism-devnet",
		1,
		consensus.ReservedPoolTreasury,
		"recipient",
		100,
	)
}

func TestNewPendingReservedTransferProposal(
	t *testing.T,
) {
	proposal :=
		testReservedTransferProposal()

	pending, err :=
		NewPendingReservedTransferProposal(
			proposal,
			100,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	if pending.Proposal.ID !=
		proposal.ID {

		t.Fatal(
			"pending proposal ID mismatch",
		)
	}

	if pending.ProposalHeight != 100 {
		t.Fatalf(
			"unexpected proposal height: %d",
			pending.ProposalHeight,
		)
	}

	if pending.DelayBlocks != 5 {
		t.Fatalf(
			"unexpected delay: %d",
			pending.DelayBlocks,
		)
	}

	if pending.ExecuteAfterHeight != 105 {
		t.Fatalf(
			"unexpected execute-after height: %d",
			pending.ExecuteAfterHeight,
		)
	}
}

func TestPendingReservedTransferProposalTimelock(
	t *testing.T,
) {
	pending, err :=
		NewPendingReservedTransferProposal(
			testReservedTransferProposal(),
			100,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	if pending.IsExecutableAt(104) {
		t.Fatal(
			"proposal should not be executable before timelock",
		)
	}

	if !pending.IsExecutableAt(105) {
		t.Fatal(
			"proposal should be executable at timelock boundary",
		)
	}

	err =
		pending.RequireExecutableAt(
			104,
		)

	if err == nil {
		t.Fatal(
			"expected early execution to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"timelock not reached",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if err :=
		pending.RequireExecutableAt(
			105,
		); err != nil {

		t.Fatalf(
			"execution at boundary rejected: %v",
			err,
		)
	}
}

func TestPendingReservedTransferProposalRejectsZeroHeight(
	t *testing.T,
) {
	_, err :=
		NewPendingReservedTransferProposal(
			testReservedTransferProposal(),
			0,
			5,
		)

	if err == nil {
		t.Fatal(
			"expected zero proposal height to be rejected",
		)
	}
}

func TestPendingReservedTransferProposalRejectsZeroDelay(
	t *testing.T,
) {
	_, err :=
		NewPendingReservedTransferProposal(
			testReservedTransferProposal(),
			100,
			0,
		)

	if err == nil {
		t.Fatal(
			"expected zero governance delay to be rejected",
		)
	}
}

func TestPendingReservedTransferProposalRejectsHeightOverflow(
	t *testing.T,
) {
	_, err :=
		NewPendingReservedTransferProposal(
			testReservedTransferProposal(),
			math.MaxUint64,
			1,
		)

	if err == nil {
		t.Fatal(
			"expected execution height overflow to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"execution height overflow",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestValidatePendingReservedTransferProposalRejectsTamperedExecutionHeight(
	t *testing.T,
) {
	pending, err :=
		NewPendingReservedTransferProposal(
			testReservedTransferProposal(),
			100,
			5,
		)

	if err != nil {
		t.Fatal(err)
	}

	pending.ExecuteAfterHeight = 104

	err =
		ValidatePendingReservedTransferProposal(
			pending,
		)

	if err == nil {
		t.Fatal(
			"expected tampered execution height to be rejected",
		)
	}

	if !strings.Contains(
		err.Error(),
		"invalid pending reserved transfer proposal execution height",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
