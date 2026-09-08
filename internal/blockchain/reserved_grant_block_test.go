package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestAddReservedGrantBlock(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	before, err :=
		bc.ReservedEmission()

	if err != nil {
		t.Fatal(err)
	}

	block, err :=
		bc.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if block.Height != 1 {
		t.Fatalf(
			"expected reserved grant block height 1, got %d",
			block.Height,
		)
	}

	if len(block.ReservedGrants) != 1 {
		t.Fatalf(
			"expected one reserved grant, got %d",
			len(block.ReservedGrants),
		)
	}

	if block.ReservedGrants[0].ID !=
		grant.ID {

		t.Fatal(
			"produced block did not preserve grant",
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"expected produced reserved grant block to validate",
		)
	}

	balance, err :=
		bc.BalanceOf(
			grant.Recipient,
		)

	if err != nil {
		t.Fatal(err)
	}

	if balance != grant.Amount {
		t.Fatalf(
			"expected grant recipient balance %d, got %d",
			grant.Amount,
			balance,
		)
	}

	after, err :=
		bc.ReservedEmission()

	if err != nil {
		t.Fatal(err)
	}

	expected :=
		before + grant.Amount

	if after != expected {
		t.Fatalf(
			"expected reserved emission %d, got %d",
			expected,
			after,
		)
	}
}

func TestAddReservedGrantBlockRejectsEmpty(
	t *testing.T,
) {
	bc, pos, validator, _ :=
		thresholdGrantBlockchain(t)

	before :=
		len(bc.Blocks)

	if _, err :=
		bc.AddReservedGrantBlock(
			nil,
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected empty reserved grant block to fail",
		)
	}

	if len(bc.Blocks) != before {
		t.Fatal(
			"failed reserved grant block mutated chain",
		)
	}
}

func TestAddReservedGrantBlockRejectsBelowThreshold(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			1,
		)

	before :=
		len(bc.Blocks)

	if _, err :=
		bc.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected below-threshold reserved grant block to fail",
		)
	}

	if len(bc.Blocks) != before {
		t.Fatal(
			"invalid reserved grant block mutated chain",
		)
	}
}

func TestAddReservedGrantBlockCopiesGrantApprovals(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	originalSignature :=
		grant.Approvals[0].Signature

	block, err :=
		bc.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	grant.Approvals[0].Signature =
		"tampered-input"

	stored :=
		&bc.Blocks[len(bc.Blocks)-1]

	if stored.ReservedGrants[0].
		Approvals[0].
		Signature != originalSignature {

		t.Fatal(
			"input grant mutation changed stored block",
		)
	}

	block.ReservedGrants[0].
		Approvals[0].
		Signature = "tampered-return"

	if stored.ReservedGrants[0].
		Approvals[0].
		Signature != originalSignature {

		t.Fatal(
			"returned block mutation changed stored block",
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"stored chain became invalid after external grant mutation",
		)
	}
}
