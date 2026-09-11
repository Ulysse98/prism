package blockchain

import (
	"testing"

	"prism/internal/reserved"
	"prism/internal/wallet"
)

func nextQueuedGovernanceProposer(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
) string {
	t.Helper()

	previous :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	proposer, err :=
		fixture.pos.SelectProposer(
			previous.Hash,
			previous.Height+1,
		)

	if err != nil {
		t.Fatal(err)
	}

	return proposer.Address
}

func TestAddAuthorityProposalBlockProducesAtActivationHeight(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
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

	block, err :=
		fixture.bc.AddAuthorityProposalBlock(
			[]reserved.AuthorityProposal{
				proposal,
			},
			nextQueuedGovernanceProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if block.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected proposal block at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			block.Height,
		)
	}

	if len(block.AuthorityProposals) != 1 {
		t.Fatalf(
			"expected one authority proposal, got %d",
			len(block.AuthorityProposals),
		)
	}

	if block.AuthorityProposals[0].ID !=
		proposal.ID {

		t.Fatal(
			"produced block contains wrong authority proposal",
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"locally produced authority proposal block failed consensus validation",
		)
	}

	state, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"locally produced authority proposal was not queued",
		)
	}

	if pending.ProposalHeight !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"unexpected proposal height: got=%d expected=%d",
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
			"unexpected execution boundary: got=%d expected=%d",
			pending.ExecuteAfterHeight,
			expectedExecuteAfter,
		)
	}
}

func TestAddAuthorityProposalBlockRejectsBeforeActivation(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

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

	before :=
		len(fixture.bc.Blocks)

	_, err =
		fixture.bc.AddAuthorityProposalBlock(
			[]reserved.AuthorityProposal{
				proposal,
			},
			nextQueuedGovernanceProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err == nil {
		t.Fatal(
			"expected authority proposal production before activation to fail",
		)
	}

	if len(fixture.bc.Blocks) != before {
		t.Fatal(
			"failed pre-activation proposal production mutated blockchain",
		)
	}

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height != 0 {
		t.Fatalf(
			"expected chain to remain at genesis, got height %d",
			last.Height,
		)
	}
}

func TestAddAuthorityExecutionBlockRejectsBeforeDelay(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
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

	_, err =
		fixture.bc.AddAuthorityProposalBlock(
			[]reserved.AuthorityProposal{
				proposal,
			},
			nextQueuedGovernanceProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	executionHeight :=
		QueuedGovernanceActivationHeight +
			reserved.DefaultGovernanceDelayBlocks

	// Advance so the next produced block is one height before
	// the exact execution boundary.
	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >=
			executionHeight-2 {

			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	before :=
		len(fixture.bc.Blocks)

	_, err =
		fixture.bc.AddAuthorityExecutionBlock(
			[]reserved.AuthorityExecution{
				reserved.NewAuthorityExecution(
					proposal.ID,
				),
			},
			nextQueuedGovernanceProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err == nil {
		t.Fatal(
			"expected authority execution before governance delay to fail",
		)
	}

	if len(fixture.bc.Blocks) != before {
		t.Fatal(
			"failed early authority execution mutated blockchain",
		)
	}

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	expectedTip :=
		executionHeight - 2

	if last.Height != expectedTip {
		t.Fatalf(
			"unexpected chain height after rejected execution: got=%d expected=%d",
			last.Height,
			expectedTip,
		)
	}

	state, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		state.GetPendingAuthorityProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"rejected early execution removed pending proposal",
		)
	}
}

func TestAddAuthorityExecutionBlockProducesAtBoundary(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
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

	proposalBlock, err :=
		fixture.bc.AddAuthorityProposalBlock(
			[]reserved.AuthorityProposal{
				proposal,
			},
			nextQueuedGovernanceProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if proposalBlock.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected proposal at height %d, got %d",
			QueuedGovernanceActivationHeight,
			proposalBlock.Height,
		)
	}

	executionHeight :=
		QueuedGovernanceActivationHeight +
			reserved.DefaultGovernanceDelayBlocks

	// Advance through the block immediately before the
	// execution boundary.
	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >=
			executionHeight-1 {

			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	executionBlock, err :=
		fixture.bc.AddAuthorityExecutionBlock(
			[]reserved.AuthorityExecution{
				reserved.NewAuthorityExecution(
					proposal.ID,
				),
			},
			nextQueuedGovernanceProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if executionBlock.Height !=
		executionHeight {

		t.Fatalf(
			"expected execution at height %d, got %d",
			executionHeight,
			executionBlock.Height,
		)
	}

	if len(executionBlock.AuthorityExecutions) != 1 {
		t.Fatalf(
			"expected one authority execution, got %d",
			len(executionBlock.AuthorityExecutions),
		)
	}

	if executionBlock.AuthorityExecutions[0].ProposalID !=
		proposal.ID {

		t.Fatal(
			"produced execution block contains wrong proposal ID",
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"locally produced boundary execution failed consensus validation",
		)
	}

	state, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
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
			proposal.Change.Pool,
			target.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !authorized {
		t.Fatal(
			"boundary execution did not activate proposed authority",
		)
	}
}
