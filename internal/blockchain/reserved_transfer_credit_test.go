package blockchain

import (
	"math"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func TestCreditReservedTransferCreditsRecipient(
	t *testing.T,
) {
	state := State{
		Balances: map[string]uint64{
			"recipient": 7,
		},
		Nonces: make(map[string]uint64),
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	if err :=
		creditReservedTransfer(
			&state,
			proposal,
		); err != nil {

		t.Fatal(err)
	}

	if got := state.Balances["recipient"]; got != 107 {

		t.Fatalf(
			"unexpected recipient balance: got=%d expected=107",
			got,
		)
	}
}

func TestCreditReservedTransferRejectsOverflow(
	t *testing.T,
) {
	state := State{
		Balances: map[string]uint64{
			"recipient": math.MaxUint64,
		},
		Nonces: make(map[string]uint64),
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			1,
		)

	if err :=
		creditReservedTransfer(
			&state,
			proposal,
		); err == nil {

		t.Fatal(
			"expected reserved transfer balance overflow",
		)
	}
}
