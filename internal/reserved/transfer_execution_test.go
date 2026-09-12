package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
)

func newReservedTransferExecutionState(
	t *testing.T,
) (
	*GovernanceState,
	*AccountingState,
	ReservedTransferProposal,
) {
	t.Helper()

	authority :=
		mustReservedTransferWallet(t)

	state, err :=
		NewGovernanceStateForChain(
			"prism-devnet",
			AuthorityPolicy{
				Treasury: []string{
					authority.Address,
				},
				TreasuryThreshold: 1,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	if err :=
		state.QueueReservedTransferProposal(
			proposal,
			"prism-devnet",
			100,
			5,
		); err != nil {

		t.Fatal(err)
	}

	return state,
		NewAccountingState(0),
		proposal
}

func TestExecuteReservedTransferProposal(
	t *testing.T,
) {
	state, accounting, proposal :=
		newReservedTransferExecutionState(t)

	executed, err :=
		state.ExecuteReservedTransferProposal(
			proposal.ID,
			"prism-devnet",
			105,
			accounting,
			consensus.DefaultSupplyPolicy(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if executed.ID != proposal.ID {
		t.Fatalf(
			"unexpected executed proposal ID: %s",
			executed.ID,
		)
	}

	if accounting.Usage.Treasury != 100 {
		t.Fatalf(
			"unexpected treasury usage: %d",
			accounting.Usage.Treasury,
		)
	}

	if _, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		); exists {

		t.Fatal(
			"executed transfer remained pending",
		)
	}
}

func TestExecuteReservedTransferProposalRejectsEarlyExecution(
	t *testing.T,
) {
	state, accounting, proposal :=
		newReservedTransferExecutionState(t)

	_, err :=
		state.ExecuteReservedTransferProposal(
			proposal.ID,
			"prism-devnet",
			104,
			accounting,
			consensus.DefaultSupplyPolicy(),
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

	if accounting.Usage.Treasury != 0 {
		t.Fatal(
			"failed execution mutated reserved accounting",
		)
	}

	if _, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"failed execution removed pending transfer",
		)
	}
}

func TestExecuteReservedTransferProposalRejectsLaterNonceFirst(
	t *testing.T,
) {
	authority :=
		mustReservedTransferWallet(t)

	state, err :=
		NewGovernanceStateForChain(
			"prism-devnet",
			AuthorityPolicy{
				Treasury: []string{
					authority.Address,
				},
				TreasuryThreshold: 1,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	first :=
		signedTreasuryTransfer(
			t,
			authority,
			1,
		)

	second :=
		signedTreasuryTransfer(
			t,
			authority,
			2,
		)

	if err :=
		state.QueueReservedTransferProposal(
			first,
			"prism-devnet",
			100,
			5,
		); err != nil {

		t.Fatal(err)
	}

	if err :=
		state.QueueReservedTransferProposal(
			second,
			"prism-devnet",
			101,
			5,
		); err != nil {

		t.Fatal(err)
	}

	accounting :=
		NewAccountingState(0)

	_, err =
		state.ExecuteReservedTransferProposal(
			second.ID,
			"prism-devnet",
			106,
			accounting,
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected later nonce to be blocked",
		)
	}

	if !strings.Contains(
		err.Error(),
		"earlier pending nonce",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestExecuteReservedTransferProposalRevalidatesActivePolicy(
	t *testing.T,
) {
	authorityA :=
		mustReservedTransferWallet(t)

	authorityB :=
		mustReservedTransferWallet(t)

	state, err :=
		NewGovernanceStateForChain(
			"prism-devnet",
			AuthorityPolicy{
				Treasury: []string{
					authorityA.Address,
				},
				TreasuryThreshold: 1,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedTreasuryTransfer(
			t,
			authorityA,
			1,
		)

	if err :=
		state.QueueReservedTransferProposal(
			proposal,
			"prism-devnet",
			100,
			5,
		); err != nil {

		t.Fatal(err)
	}

	// Simulate a valid governance change that removed authority A
	// during the transfer's timelock.
	state.CurrentPolicy =
		AuthorityPolicy{
			Treasury: []string{
				authorityB.Address,
			},
			TreasuryThreshold: 1,
		}

	accounting :=
		NewAccountingState(0)

	_, err =
		state.ExecuteReservedTransferProposal(
			proposal.ID,
			"prism-devnet",
			105,
			accounting,
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected removed authority to invalidate execution",
		)
	}

	if !strings.Contains(
		err.Error(),
		"not authorized",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if _, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"failed execution removed pending transfer",
		)
	}
}

func TestExecuteReservedTransferProposalBudgetFailureKeepsPending(
	t *testing.T,
) {
	state, accounting, proposal :=
		newReservedTransferExecutionState(t)

	accounting.Usage.Treasury =
		consensus.TreasuryAllocation

	_, err :=
		state.ExecuteReservedTransferProposal(
			proposal.ID,
			"prism-devnet",
			105,
			accounting,
			consensus.DefaultSupplyPolicy(),
		)

	if err == nil {
		t.Fatal(
			"expected exhausted budget to reject execution",
		)
	}

	if _, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"failed execution removed pending transfer",
		)
	}
}
