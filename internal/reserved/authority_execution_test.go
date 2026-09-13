package reserved

import "testing"

func TestAuthorityExecutionValidates(
	t *testing.T,
) {
	execution :=
		NewAuthorityExecution(
			"proposal-123",
		)

	if err :=
		ValidateAuthorityExecution(
			execution,
		); err != nil {

		t.Fatal(err)
	}
}

func TestAuthorityExecutionRejectsEmptyProposalID(
	t *testing.T,
) {
	execution :=
		NewAuthorityExecution("")

	if err :=
		ValidateAuthorityExecution(
			execution,
		); err == nil {

		t.Fatal(
			"expected empty proposal ID to fail",
		)
	}
}