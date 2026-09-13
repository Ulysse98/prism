package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func TestGovernanceStateExecutesAuthorityProposalAtBoundary(
	t *testing.T,
) {
	state,
		proposal,
		target,
		authorities,
		chainID :=
		governanceProposalFixture(
			t,
			1,
		)

	approveGovernanceProposal(
		t,
		&proposal,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	if err := state.ExecuteAuthorityProposal(
		proposal.ID,
		chainID,
		15,
	); err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		); exists {

		t.Fatal(
			"executed proposal remained pending",
		)
	}

	authorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"executed proposal did not activate authority",
		)
	}

	err =
		state.Replay.ValidateAuthorityChangeNext(
			proposal.Change,
		)

	if err == nil {
		t.Fatal(
			"executed proposal did not consume replay protection",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already used",
	) {
		t.Fatalf(
			"unexpected replay error: %v",
			err,
		)
	}
}

func TestGovernanceStateRejectsExecutionBeforeTimelock(
	t *testing.T,
) {
	state,
		proposal,
		target,
		authorities,
		chainID :=
		governanceProposalFixture(
			t,
			1,
		)

	approveGovernanceProposal(
		t,
		&proposal,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	err :=
		state.ExecuteAuthorityProposal(
			proposal.ID,
			chainID,
			14,
		)

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

	if _, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"failed execution removed pending proposal",
		)
	}

	authorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if authorized {
		t.Fatal(
			"failed execution activated authority",
		)
	}

	if err :=
		state.Replay.ValidateAuthorityChangeNext(
			proposal.Change,
		); err != nil {

		t.Fatalf(
			"failed execution consumed replay protection: %v",
			err,
		)
	}
}

func TestGovernanceStateExecutesPendingProposalsInNonceOrder(
	t *testing.T,
) {
	state,
		first,
		firstTarget,
		authorities,
		chainID :=
		governanceProposalFixture(
			t,
			2,
		)

	approveGovernanceProposal(
		t,
		&first,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		first,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	secondTarget, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	second :=
		NewAuthorityProposal(
			chainID,
			3,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			secondTarget.Address,
		)

	approveGovernanceProposal(
		t,
		&second,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		second,
		chainID,
		11,
		4,
	); err != nil {
		t.Fatal(err)
	}

	// Both proposals are executable at height 15, but nonce 3
	// must not jump ahead of nonce 2.
	err =
		state.ExecuteAuthorityProposal(
			second.ID,
			chainID,
			15,
		)

	if err == nil {
		t.Fatal(
			"expected higher nonce execution to fail while lower nonce is pending",
		)
	}

	if !strings.Contains(
		err.Error(),
		"earlier pending nonce",
	) {
		t.Fatalf(
			"unexpected ordering error: %v",
			err,
		)
	}

	if err := state.ExecuteAuthorityProposal(
		first.ID,
		chainID,
		15,
	); err != nil {
		t.Fatal(err)
	}

	if err := state.ExecuteAuthorityProposal(
		second.ID,
		chainID,
		15,
	); err != nil {
		t.Fatal(err)
	}

	firstAuthorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			firstTarget.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !firstAuthorized {
		t.Fatal(
			"first ordered proposal did not activate authority",
		)
	}

	secondAuthorized, err :=
		state.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			secondTarget.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !secondAuthorized {
		t.Fatal(
			"second ordered proposal did not activate authority",
		)
	}

	if len(state.PendingProposals) != 0 {
		t.Fatalf(
			"expected empty pending queue after executions, got %d",
			len(state.PendingProposals),
		)
	}
}

func TestGovernanceStateRejectsUnknownProposalExecution(
	t *testing.T,
) {
	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	state, err :=
		NewGovernanceStateForChain(
			"prism-execution-test",
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

	err =
		state.ExecuteAuthorityProposal(
			"unknown-proposal",
			"prism-execution-test",
			100,
		)

	if err == nil {
		t.Fatal(
			"expected unknown proposal execution to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"not pending",
	) {
		t.Fatalf(
			"unexpected unknown proposal error: %v",
			err,
		)
	}
}

func TestGovernanceStateFailedExecutionKeepsProposalPending(
	t *testing.T,
) {
	state,
		first,
		_,
		authorities,
		chainID :=
		governanceProposalFixture(
			t,
			1,
		)

	target :=
		first.Change.Authority

	approveGovernanceProposal(
		t,
		&first,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		first,
		chainID,
		10,
		5,
	); err != nil {
		t.Fatal(err)
	}

	second :=
		NewAuthorityProposal(
			chainID,
			2,
			consensus.ReservedPoolTreasury,
			AuthorityChangeAdd,
			target,
		)

	approveGovernanceProposal(
		t,
		&second,
		authorities...,
	)

	if err := state.QueueAuthorityProposal(
		second,
		chainID,
		11,
		5,
	); err != nil {
		t.Fatal(err)
	}

	if err := state.ExecuteAuthorityProposal(
		first.ID,
		chainID,
		15,
	); err != nil {
		t.Fatal(err)
	}

	err :=
		state.ExecuteAuthorityProposal(
			second.ID,
			chainID,
			16,
		)

	if err == nil {
		t.Fatal(
			"expected structurally invalid execution to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already configured",
	) {
		t.Fatalf(
			"unexpected execution failure: %v",
			err,
		)
	}

	if _, exists :=
		state.GetPendingAuthorityProposal(
			second.ID,
		); !exists {

		t.Fatal(
			"failed execution removed proposal from pending queue",
		)
	}

	if err :=
		state.Replay.ValidateAuthorityChangeNext(
			second.Change,
		); err != nil {

		t.Fatalf(
			"failed execution consumed replay protection: %v",
			err,
		)
	}
}
