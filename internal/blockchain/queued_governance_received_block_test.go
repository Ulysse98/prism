package blockchain

import (
	"testing"

	"prism/internal/reserved"
	"prism/internal/wallet"
)

func queuedGovernanceReceiver(
	fixture queuedGovernanceConsensusFixture,
) *Blockchain {
	blocks := append(
		[]Block(nil),
		fixture.bc.Blocks...,
	)

	lockedStakes := map[string]uint64{
		fixture.validator.Address: 10,
	}

	return &Blockchain{
		Blocks:       blocks,
		LockedStakes: lockedStakes,
		Config:       fixture.bc.Config,
	}
}

func TestAppendValidatedBlockAcceptsQueuedGovernanceProposal(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	receiver :=
		queuedGovernanceReceiver(
			fixture,
		)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedQueuedGovernanceProposal(
			t,
			fixture,
			1,
			target.Address,
		)

	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	remoteBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if err := receiver.AppendValidatedBlock(
		remoteBlock,
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	if len(receiver.Blocks) != 2 {
		t.Fatalf(
			"expected receiver to contain 2 blocks, got %d",
			len(receiver.Blocks),
		)
	}

	if receiver.Blocks[1].Hash != remoteBlock.Hash {
		t.Fatal(
			"receiver did not preserve exact queued governance block hash",
		)
	}

	state, err :=
		receiver.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"received queued governance proposal was not reconstructed",
		)
	}

	if pending.ProposalHeight != 1 {
		t.Fatalf(
			"unexpected received proposal height: got=%d expected=1",
			pending.ProposalHeight,
		)
	}

	if pending.ExecuteAfterHeight != 6 {
		t.Fatalf(
			"unexpected received execution boundary: got=%d expected=6",
			pending.ExecuteAfterHeight,
		)
	}
}

func TestAppendValidatedBlockRejectsEarlyQueuedGovernanceExecution(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	receiver :=
		queuedGovernanceReceiver(
			fixture,
		)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedQueuedGovernanceProposal(
			t,
			fixture,
			1,
			target.Address,
		)

	// Proposal at height 1.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	proposalBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if err := receiver.AppendValidatedBlock(
		proposalBlock,
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	// Copy valid filler blocks through height 4.
	for len(fixture.bc.Blocks)-1 < 4 {
		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)

		remoteBlock :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if err := receiver.AppendValidatedBlock(
			remoteBlock,
			fixture.pos,
		); err != nil {
			t.Fatal(err)
		}
	}

	receiverHeight :=
		receiver.Blocks[len(receiver.Blocks)-1].Height

	if receiverHeight != 4 {
		t.Fatalf(
			"expected receiver at height 4 before early execution, got %d",
			receiverHeight,
		)
	}

	// Source constructs an execution at height 5.
	// It is hash-valid but consensus-invalid because the boundary is 6.
	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			proposal.ID,
		),
	)

	earlyExecutionBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if earlyExecutionBlock.Height != 5 {
		t.Fatalf(
			"expected early execution block at height 5, got %d",
			earlyExecutionBlock.Height,
		)
	}

	if err := receiver.AppendValidatedBlock(
		earlyExecutionBlock,
		fixture.pos,
	); err == nil {
		t.Fatal(
			"expected receiver to reject early governance execution",
		)
	}

	// Failed remote validation must not mutate the receiver.
	receiverHeight =
		receiver.Blocks[len(receiver.Blocks)-1].Height

	if receiverHeight != 4 {
		t.Fatalf(
			"receiver mutated after rejecting early governance execution: height=%d",
			receiverHeight,
		)
	}

	state, err :=
		receiver.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"rejected remote execution removed pending proposal",
		)
	}
}

func TestAppendValidatedBlockAcceptsQueuedGovernanceExecutionAtBoundary(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	receiver :=
		queuedGovernanceReceiver(
			fixture,
		)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedQueuedGovernanceProposal(
			t,
			fixture,
			1,
			target.Address,
		)

	// Proposal at height 1.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	if err := receiver.AppendValidatedBlock(
		fixture.bc.Blocks[1],
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	// Propagate filler blocks through height 5.
	for len(fixture.bc.Blocks)-1 < 5 {
		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)

		remoteBlock :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if err := receiver.AppendValidatedBlock(
			remoteBlock,
			fixture.pos,
		); err != nil {
			t.Fatal(err)
		}
	}

	// Execution at exact boundary height 6.
	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			proposal.ID,
		),
	)

	executionBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if executionBlock.Height != 6 {
		t.Fatalf(
			"expected execution block at height 6, got %d",
			executionBlock.Height,
		)
	}

	if err := receiver.AppendValidatedBlock(
		executionBlock,
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	if !receiver.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"receiver chain became invalid after valid queued governance execution",
		)
	}

	state, err :=
		receiver.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		); exists {

		t.Fatal(
			"received executed proposal remained pending",
		)
	}

	authorized, err :=
		state.CurrentPolicy.IsAuthorized(
			proposal.Change.Pool,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"received boundary execution did not activate authority",
		)
	}
}
