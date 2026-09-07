package blockchain

import (
	"math"
	"testing"

	"prism/internal/reserved"
)

func TestCreditReservedAuthorizationCreditsRecipient(
	t *testing.T,
) {
	state :=
		&State{
			Balances: map[string]uint64{
				"recipient": 25,
			},
			Nonces: make(map[string]uint64),
		}

	authorization :=
		reserved.Authorization{
			Recipient: "recipient",
			Amount:    100,
		}

	if err :=
		creditReservedAuthorization(
			state,
			authorization,
		); err != nil {

		t.Fatal(err)
	}

	if state.Balances["recipient"] != 125 {
		t.Fatalf(
			"expected recipient balance 125, got %d",
			state.Balances["recipient"],
		)
	}
}

func TestCreditReservedAuthorizationCreatesRecipientBalance(
	t *testing.T,
) {
	state :=
		&State{
			Balances: make(map[string]uint64),
			Nonces:   make(map[string]uint64),
		}

	authorization :=
		reserved.Authorization{
			Recipient: "recipient",
			Amount:    50,
		}

	if err :=
		creditReservedAuthorization(
			state,
			authorization,
		); err != nil {

		t.Fatal(err)
	}

	if state.Balances["recipient"] != 50 {
		t.Fatalf(
			"expected recipient balance 50, got %d",
			state.Balances["recipient"],
		)
	}
}

func TestCreditReservedAuthorizationRejectsOverflow(
	t *testing.T,
) {
	state :=
		&State{
			Balances: map[string]uint64{
				"recipient": math.MaxUint64,
			},
		}

	authorization :=
		reserved.Authorization{
			Recipient: "recipient",
			Amount:    1,
		}

	if err :=
		creditReservedAuthorization(
			state,
			authorization,
		); err == nil {

		t.Fatal(
			"expected reserved balance overflow to fail",
		)
	}
}

func TestCreditReservedAuthorizationRejectsNilState(
	t *testing.T,
) {
	authorization :=
		reserved.Authorization{
			Recipient: "recipient",
			Amount:    1,
		}

	if err :=
		creditReservedAuthorization(
			nil,
			authorization,
		); err == nil {

		t.Fatal(
			"expected nil state to fail",
		)
	}
}

func TestCreditReservedAuthorizationRejectsNilBalances(
	t *testing.T,
) {
	state :=
		&State{}

	authorization :=
		reserved.Authorization{
			Recipient: "recipient",
			Amount:    1,
		}

	if err :=
		creditReservedAuthorization(
			state,
			authorization,
		); err == nil {

		t.Fatal(
			"expected nil balances to fail",
		)
	}
}
