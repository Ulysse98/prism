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

	// Bring the source chain to height 2 so the next block
	// is exactly the v0.29 activation height.
	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	// Receiver begins from the exact same historical chain.
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

	if remoteBlock.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected proposal block at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			remoteBlock.Height,
		)
	}

	if err := receiver.AppendValidatedBlock(
		remoteBlock,
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	receiverTip :=
		receiver.Blocks[len(receiver.Blocks)-1]

	if receiverTip.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected receiver height %d, got %d",
			QueuedGovernanceActivationHeight,
			receiverTip.Height,
		)
	}

	if receiverTip.Hash != remoteBlock.Hash {
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

	if pending.ProposalHeight !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"unexpected received proposal height: got=%d expected=%d",
			pending.ProposalHeight,
			QueuedGovernanceActivationHeight,
		)
	}

	expectedExecuteAfter :=
		QueuedGovernanceActivationHeight +
			reserved.DefaultGovernanceDelayBlocks

	if pending.ExecuteAfterHeight !=
		expectedExecuteAfter {

		t.Fatalf(
			"unexpected received execution boundary: got=%d expected=%d",
			pending.ExecuteAfterHeight,
			expectedExecuteAfter,
		)
	}
}

func TestAppendValidatedBlockRejectsEarlyQueuedGovernanceExecution(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

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

	// Proposal at activation height 3.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	proposalBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if proposalBlock.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected proposal at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			proposalBlock.Height,
		)
	}

	if err := receiver.AppendValidatedBlock(
		proposalBlock,
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	// Execution boundary is:
	// 3 + 5 = 8.
	//
	// Propagate valid filler blocks through height 6 so the
	// attempted execution lands at height 7.
	for {
		sourceTip :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if sourceTip.Height >= 6 {
			break
		}

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

	receiverTip :=
		receiver.Blocks[len(receiver.Blocks)-1]

	if receiverTip.Height != 6 {
		t.Fatalf(
			"expected receiver at height 6 before early execution, got %d",
			receiverTip.Height,
		)
	}

	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			proposal.ID,
		),
	)

	earlyExecutionBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if earlyExecutionBlock.Height != 7 {
		t.Fatalf(
			"expected early execution block at height 7, got %d",
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

	// Failed validation must not mutate the receiver.
	receiverTip =
		receiver.Blocks[len(receiver.Blocks)-1]

	if receiverTip.Height != 6 {
		t.Fatalf(
			"receiver mutated after rejecting early governance execution: height=%d",
			receiverTip.Height,
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

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

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

	// Proposal at activation height 3.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	proposalBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if proposalBlock.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected proposal at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			proposalBlock.Height,
		)
	}

	if err := receiver.AppendValidatedBlock(
		proposalBlock,
		fixture.pos,
	); err != nil {
		t.Fatal(err)
	}

	// Propagate filler blocks through height 7.
	for {
		sourceTip :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if sourceTip.Height >= 7 {
			break
		}

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

	receiverTip :=
		receiver.Blocks[len(receiver.Blocks)-1]

	if receiverTip.Height != 7 {
		t.Fatalf(
			"expected receiver at height 7 before boundary execution, got %d",
			receiverTip.Height,
		)
	}

	// Exact execution boundary:
	// activation height 3 + governance delay 5 = height 8.
	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			proposal.ID,
		),
	)

	executionBlock :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	expectedExecutionHeight :=
		QueuedGovernanceActivationHeight +
			reserved.DefaultGovernanceDelayBlocks

	if executionBlock.Height !=
		expectedExecutionHeight {

		t.Fatalf(
			"expected execution block at height %d, got %d",
			expectedExecutionHeight,
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
