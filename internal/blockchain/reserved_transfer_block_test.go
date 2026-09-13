package blockchain

import (
	"testing"

	"prism/internal/reserved"
	"prism/internal/wallet"
)

func expectedReservedTransferBuilderProposer(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
) string {
	t.Helper()

	previous :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	proposer, err :=
		fixture.pos.SelectProposer(
			previous.Hash,
			nextHeight,
		)

	if err != nil {
		t.Fatal(err)
	}

	return proposer.Address
}

func TestAddReservedTransferProposalBlockRejectsBeforeActivation(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	before :=
		len(fixture.bc.Blocks)

	_, err :=
		fixture.bc.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err == nil {
		t.Fatal(
			"expected proposal builder before v0.30 activation to fail",
		)
	}

	if len(fixture.bc.Blocks) != before {
		t.Fatal(
			"failed proposal builder mutated blockchain",
		)
	}
}

func TestAddReservedTransferProposalBlockAppendsValidatedBlock(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToGovernedReservedTransferActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	block, err :=
		fixture.bc.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if block.Height !=
		GovernedReservedTransferActivationHeight {

		t.Fatalf(
			"unexpected proposal block height: got=%d expected=%d",
			block.Height,
			GovernedReservedTransferActivationHeight,
		)
	}

	if len(block.ReservedTransferProposals) != 1 {
		t.Fatalf(
			"expected one reserved transfer proposal, got %d",
			len(block.ReservedTransferProposals),
		)
	}

	if block.ReservedTransferProposals[0].ID !=
		proposal.ID {

		t.Fatal(
			"produced block did not preserve transfer proposal",
		)
	}

	stored :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if stored.Hash != block.Hash {
		t.Fatal(
			"produced proposal block was not appended",
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"proposal builder produced invalid chain",
		)
	}
}

func TestAddReservedTransferProposalBlockRejectsWrongProposer(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToGovernedReservedTransferActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	wrong, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	before :=
		len(fixture.bc.Blocks)

	_, err =
		fixture.bc.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			wrong.Address,
			fixture.pos,
		)

	if err == nil {
		t.Fatal(
			"expected wrong proposer to be rejected",
		)
	}

	if len(fixture.bc.Blocks) != before {
		t.Fatal(
			"wrong proposer mutated blockchain",
		)
	}
}

func TestAddReservedTransferProposalBlockClonesApprovals(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToGovernedReservedTransferActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	if len(proposal.Approvals) == 0 {
		t.Fatal(
			"test proposal has no approvals",
		)
	}

	originalSignature :=
		proposal.Approvals[0].Signature

	block, err :=
		fixture.bc.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	// Mutating caller-owned input must not mutate canonical storage.
	proposal.Approvals[0].Signature =
		"tampered-input"

	stored :=
		&fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if stored.ReservedTransferProposals[0].
		Approvals[0].
		Signature != originalSignature {

		t.Fatal(
			"caller mutation altered stored proposal approvals",
		)
	}

	// Mutating the returned block must not mutate canonical storage either.
	block.ReservedTransferProposals[0].
		Approvals[0].
		Signature = "tampered-return"

	if stored.ReservedTransferProposals[0].
		Approvals[0].
		Signature != originalSignature {

		t.Fatal(
			"returned block mutation altered stored proposal approvals",
		)
	}
}

func TestAddReservedTransferExecutionBlockRejectsBeforeTimelock(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToGovernedReservedTransferActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	_, err :=
		fixture.bc.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	beforeBlocks :=
		len(fixture.bc.Blocks)

	_, err =
		fixture.bc.AddReservedTransferExecutionBlock(
			[]reserved.ReservedTransferExecution{
				reserved.NewReservedTransferExecution(
					proposal.ID,
				),
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err == nil {
		t.Fatal(
			"expected execution before timelock to fail",
		)
	}

	if len(fixture.bc.Blocks) != beforeBlocks {
		t.Fatal(
			"failed early execution mutated blockchain",
		)
	}

	governance, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		governance.GetPendingReservedTransferProposal(
			proposal.ID,
		); !exists {

		t.Fatal(
			"early failed execution removed pending proposal",
		)
	}
}

func TestAddReservedTransferExecutionBlockExecutesAtBoundary(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToGovernedReservedTransferActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	proposalBlock, err :=
		fixture.bc.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	expectedExecutionHeight :=
		proposalBlock.Height +
			reserved.DefaultGovernanceDelayBlocks

	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >=
			expectedExecutionHeight-1 {

			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	beforeBalance, err :=
		fixture.bc.BalanceOf(
			fixture.sink.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	execution :=
		reserved.NewReservedTransferExecution(
			proposal.ID,
		)

	block, err :=
		fixture.bc.AddReservedTransferExecutionBlock(
			[]reserved.ReservedTransferExecution{
				execution,
			},
			expectedReservedTransferBuilderProposer(
				t,
				fixture,
			),
			fixture.pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if block.Height != expectedExecutionHeight {
		t.Fatalf(
			"unexpected execution block height: got=%d expected=%d",
			block.Height,
			expectedExecutionHeight,
		)
	}

	if len(block.ReservedTransferExecutions) != 1 {
		t.Fatalf(
			"expected one reserved transfer execution, got %d",
			len(block.ReservedTransferExecutions),
		)
	}

	if block.ReservedTransferExecutions[0].ProposalID !=
		proposal.ID {

		t.Fatal(
			"produced execution block did not preserve proposal ID",
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"execution builder produced invalid chain",
		)
	}

	afterBalance, err :=
		fixture.bc.BalanceOf(
			fixture.sink.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	expectedBalance :=
		beforeBalance + proposal.Amount

	if afterBalance != expectedBalance {
		t.Fatalf(
			"execution builder did not credit recipient: before=%d amount=%d after=%d expected=%d",
			beforeBalance,
			proposal.Amount,
			afterBalance,
			expectedBalance,
		)
	}

	accounting, err :=
		fixture.bc.GetReservedAccountingState()

	if err != nil {
		t.Fatal(err)
	}

	if accounting.Usage.Treasury !=
		proposal.Amount {

		t.Fatalf(
			"unexpected treasury usage: got=%d expected=%d",
			accounting.Usage.Treasury,
			proposal.Amount,
		)
	}

	governance, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		governance.GetPendingReservedTransferProposal(
			proposal.ID,
		); exists {

		t.Fatal(
			"executed proposal remained pending",
		)
	}
}
