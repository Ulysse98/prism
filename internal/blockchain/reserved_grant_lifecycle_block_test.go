package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestAddReservedGrantBlockRejectsBeforeLifecycleActivation(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedLifecycleGrantForBlockchain(
			t,
			bc,
			authorities,
			1,
			2,
			10,
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
			"expected producer to reject grant before lifecycle activation",
		)
	}

	if len(bc.Blocks) != before {
		t.Fatal(
			"premature lifecycle grant mutated chain",
		)
	}
}

func TestAddReservedGrantBlockAcceptsLifecycleBoundary(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	grant :=
		signedLifecycleGrantForBlockchain(
			t,
			bc,
			authorities,
			1,
			1,
			1,
		)

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
			"expected lifecycle block height 1, got %d",
			block.Height,
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"produced lifecycle-boundary grant block failed validation",
		)
	}
}

func TestAddReservedGrantBlockRejectsExpiredLifecycleGrant(
	t *testing.T,
) {
	bc, pos, validator, authorities :=
		thresholdGrantBlockchain(t)

	first :=
		signedThresholdGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
		)

	appendThresholdGrantTestBlock(
		bc,
		validator,
		first,
	)

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"setup block failed validation",
		)
	}

	expired :=
		signedLifecycleGrantForBlockchain(
			t,
			bc,
			authorities,
			2,
			0,
			1,
		)

	before :=
		len(bc.Blocks)

	if _, err :=
		bc.AddReservedGrantBlock(
			[]reserved.Grant{
				expired,
			},
			validator.Address,
			pos,
		); err == nil {

		t.Fatal(
			"expected producer to reject expired lifecycle grant",
		)
	}

	if len(bc.Blocks) != before {
		t.Fatal(
			"expired lifecycle grant mutated chain",
		)
	}
}
