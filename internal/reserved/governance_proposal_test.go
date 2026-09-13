package reserved

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func governanceProposalFixture(
	t *testing.T,
	nonce uint64,
) (
	*GovernanceState,
	AuthorityProposal,
	*wallet.Wallet,
	[]*wallet.Wallet,
	string,
) {
	t.Helper()

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	policy := AuthorityPolicy{
		Treasury: []string{
			authorityA.Address,
			authorityB.Address,
		},
		TreasuryThreshold: 2,
	}

	const chainID = "prism-queued-governance-test"

	state, err := NewGovernanceStateForChain(
		chainID,
		policy,
	)
	if err != nil {
		t.Fatal(err)
	}

	proposal := NewAuthorityProposal(
		chainID,
		nonce,
		consensus.ReservedPoolTreasury,
		AuthorityChangeAdd,
		target.Address,
	)

	return state,
		proposal,
		target,
		[]*wallet.Wallet{
			authorityA,
			authorityB,
		},
		chainID
}

func approveGovernanceProposal(
	t *testing.T,
	proposal *AuthorityProposal,
	authorities ...*wallet.Wallet,
) {
	t.Helper()

	for _, authority := range authorities {
		if err := proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGovernanceStateQueuesAuthorityProposal(
	t *testing.T,
) {
	state,
		proposal,
		target,
		authorities,
		chainID := governanceProposalFixture(
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

	if len(state.PendingProposals) != 1 {
		t.Fatalf(
			"expected one pending proposal, got %d",
			len(state.PendingProposals),
		)
	}

	pending, exists := state.GetPendingAuthorityProposal(
		proposal.ID,
	)
	if !exists {
		t.Fatal(
			"queued proposal was not found",
		)
	}

	if pending.ProposalHeight != 10 {
		t.Fatalf(
			"unexpected proposal height: got=%d expected=10",
			pending.ProposalHeight,
		)
	}

	if pending.DelayBlocks != 5 {
		t.Fatalf(
			"unexpected governance delay: got=%d expected=5",
			pending.DelayBlocks,
		)
	}

	if pending.ExecuteAfterHeight != 15 {
		t.Fatalf(
			"unexpected execution height: got=%d expected=15",
			pending.ExecuteAfterHeight,
		)
	}

	authorized, err := state.CurrentPolicy.IsAuthorized(
		consensus.ReservedPoolTreasury,
		target.Address,
	)
	if err != nil {
		t.Fatal(err)
	}

	if authorized {
		t.Fatal(
			"pending proposal activated authority before execution",
		)
	}
}

func TestGovernanceStateQueueDoesNotConsumeReplayState(
	t *testing.T,
) {
	state,
		proposal,
		_,
		authorities,
		chainID := governanceProposalFixture(
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

	if _, exists :=
		state.Replay.usedAuthorityChangeIDs[proposal.Change.ID]; exists {

		t.Fatal(
			"queueing proposal consumed authority-change ID",
		)
	}

	key := authorityChangeReplayKey{
		Pool: proposal.Change.Pool,
	}

	if _, exists :=
		state.Replay.lastAuthorityChangeNonce[key]; exists {

		t.Fatal(
			"queueing proposal consumed authority-change nonce",
		)
	}
}

func TestGovernanceStateRejectsProposalBelowThreshold(
	t *testing.T,
) {
	state,
		proposal,
		_,
		authorities,
		chainID := governanceProposalFixture(
		t,
		1,
	)

	approveGovernanceProposal(
		t,
		&proposal,
		authorities[0],
	)

	err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		10,
		5,
	)

	if err == nil {
		t.Fatal(
			"expected proposal below threshold to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"threshold not met",
	) {
		t.Fatalf(
			"unexpected threshold error: %v",
			err,
		)
	}

	if len(state.PendingProposals) != 0 {
		t.Fatal(
			"rejected proposal entered pending queue",
		)
	}
}

func TestGovernanceStateRejectsDuplicatePendingProposal(
	t *testing.T,
) {
	state,
		proposal,
		_,
		authorities,
		chainID := governanceProposalFixture(
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

	err := state.QueueAuthorityProposal(
		proposal,
		chainID,
		11,
		5,
	)

	if err == nil {
		t.Fatal(
			"expected duplicate pending proposal to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already pending",
	) {
		t.Fatalf(
			"unexpected duplicate error: %v",
			err,
		)
	}

	if len(state.PendingProposals) != 1 {
		t.Fatalf(
			"duplicate proposal changed pending count: %d",
			len(state.PendingProposals),
		)
	}
}

func TestGovernanceStateRejectsNonIncreasingPendingNonce(
	t *testing.T,
) {
	state,
		first,
		_,
		authorities,
		chainID := governanceProposalFixture(
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

	second := NewAuthorityProposal(
		chainID,
		1,
		consensus.ReservedPoolTreasury,
		AuthorityChangeAdd,
		secondTarget.Address,
	)

	approveGovernanceProposal(
		t,
		&second,
		authorities...,
	)

	err = state.QueueAuthorityProposal(
		second,
		chainID,
		11,
		5,
	)

	if err == nil {
		t.Fatal(
			"expected non-increasing pending nonce to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"nonce is not increasing",
	) {
		t.Fatalf(
			"unexpected nonce error: %v",
			err,
		)
	}
}

func TestGovernanceStateRejectsProposalForWrongChain(
	t *testing.T,
) {
	state,
		proposal,
		_,
		authorities,
		_ := governanceProposalFixture(
		t,
		1,
	)

	approveGovernanceProposal(
		t,
		&proposal,
		authorities...,
	)

	err := state.QueueAuthorityProposal(
		proposal,
		"wrong-chain",
		10,
		5,
	)

	if err == nil {
		t.Fatal(
			"expected wrong-chain proposal to fail",
		)
	}

	if len(state.PendingProposals) != 0 {
		t.Fatal(
			"wrong-chain proposal entered pending queue",
		)
	}
}

func TestGovernanceStatePendingProposalIsCopied(
	t *testing.T,
) {
	state,
		proposal,
		_,
		authorities,
		chainID := governanceProposalFixture(
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

	proposal.Change.Approvals[0].Signature =
		"tampered"

	stored, exists := state.GetPendingAuthorityProposal(
		proposal.ID,
	)
	if !exists {
		t.Fatal(
			"pending proposal not found",
		)
	}

	if stored.Proposal.Change.Approvals[0].Signature ==
		"tampered" {

		t.Fatal(
			"caller mutation changed stored pending proposal",
		)
	}
}
