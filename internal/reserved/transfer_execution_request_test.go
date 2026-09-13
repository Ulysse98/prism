package reserved

import "testing"

func TestNewReservedTransferExecution(
	t *testing.T,
) {
	execution :=
		NewReservedTransferExecution(
			"proposal-id",
		)

	if execution.ProposalID !=
		"proposal-id" {

		t.Fatalf(
			"unexpected proposal ID: %s",
			execution.ProposalID,
		)
	}
}

func TestValidateReservedTransferExecution(
	t *testing.T,
) {
	execution :=
		NewReservedTransferExecution(
			"proposal-id",
		)

	if err :=
		ValidateReservedTransferExecution(
			execution,
		); err != nil {

		t.Fatal(err)
	}
}

func TestValidateReservedTransferExecutionRejectsEmptyProposalID(
	t *testing.T,
) {
	err :=
		ValidateReservedTransferExecution(
			ReservedTransferExecution{},
		)

	if err == nil {
		t.Fatal(
			"expected empty proposal ID to be rejected",
		)
	}
}
