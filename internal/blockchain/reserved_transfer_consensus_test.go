package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func signedReservedTransferConsensusProposal(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	nonce uint64,
) reserved.ReservedTransferProposal {
	t.Helper()

	chainID, err :=
		fixture.bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			chainID,
			nonce,
			consensus.ReservedPoolTreasury,
			fixture.sink.Address,
			100,
		)

	for _, authority := range fixture.authorities {

		if err :=
			proposal.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	return proposal
}

func appendReservedTransferProposalConsensusBlock(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	proposal reserved.ReservedTransferProposal,
) {
	t.Helper()

	bc := fixture.bc

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

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

	block := Block{
		Height: nextHeight,
		Timestamp: time.Unix(
			int64(nextHeight+3000),
			0,
		).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       consensus.DefaultProposerReward,
		ReservedTransferProposals: []reserved.ReservedTransferProposal{
			proposal,
		},
	}

	block.Hash =
		CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)
}

func TestReservedTransferProposalParticipatesInConsensus(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected reserved transfer proposal to pass consensus",
		)
	}

	state, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		state.GetPendingReservedTransferProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"consensus reconstruction lost pending reserved transfer",
		)
	}

	if pending.ProposalHeight !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"unexpected proposal height: %d",
			pending.ProposalHeight,
		)
	}

	expectedExecuteAfter :=
		QueuedGovernanceActivationHeight +
			reserved.DefaultGovernanceDelayBlocks

	if pending.ExecuteAfterHeight !=
		expectedExecuteAfter {

		t.Fatalf(
			"unexpected execute-after height: got=%d expected=%d",
			pending.ExecuteAfterHeight,
			expectedExecuteAfter,
		)
	}
}

func TestReservedTransferProposalBelowQuorumFailsConsensus(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	chainID, err :=
		fixture.bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			fixture.sink.Address,
			100,
		)

	authority :=
		fixture.authorities[0]

	if err :=
		proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected below-quorum reserved transfer proposal to fail consensus",
		)
	}
}

func appendReservedTransferExecutionConsensusBlock(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	execution reserved.ReservedTransferExecution,
) {
	t.Helper()

	bc := fixture.bc

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

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

	block := Block{
		Height: nextHeight,
		Timestamp: time.Unix(
			int64(nextHeight+4000),
			0,
		).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       consensus.DefaultProposerReward,
		ReservedTransferExecutions: []reserved.ReservedTransferExecution{
			execution,
		},
	}

	block.Hash =
		CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)
}

func TestReservedTransferExecutionFailsConsensusBeforeDelay(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	// Proposal is included at height 3.
	// With the five-block governance delay, execution starts at height 8.
	// Advance only through height 6 so the execution lands at height 7.
	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >= 6 {
			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	appendReservedTransferExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewReservedTransferExecution(
			proposal.ID,
		),
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height != 7 {
		t.Fatalf(
			"expected early transfer execution at height 7, got %d",
			last.Height,
		)
	}

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected reserved transfer execution before timelock to fail consensus",
		)
	}

	if _, err :=
		fixture.bc.GetReservedAccountingState(); err == nil {

		t.Fatal(
			"expected accounting replay to reject early reserved transfer execution",
		)
	}
}

func TestReservedTransferExecutionPassesConsensusAtBoundary(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	// Advance through height 7 so execution lands exactly at height 8.
	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >= 7 {
			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	appendReservedTransferExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewReservedTransferExecution(
			proposal.ID,
		),
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height != 8 {
		t.Fatalf(
			"expected boundary transfer execution at height 8, got %d",
			last.Height,
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected reserved transfer execution at timelock boundary to pass consensus",
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
			"executed reserved transfer remained pending",
		)
	}

	accounting, err :=
		fixture.bc.GetReservedAccountingState()

	if err != nil {
		t.Fatal(err)
	}

	if accounting.Usage.Treasury != 100 {
		t.Fatalf(
			"unexpected treasury usage after execution: got=%d expected=100",
			accounting.Usage.Treasury,
		)
	}
}

func TestReservedTransferUnknownExecutionFailsConsensus(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	appendReservedTransferExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewReservedTransferExecution(
			"unknown-transfer-proposal",
		),
	)

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected unknown reserved transfer execution to fail consensus",
		)
	}
}

func TestReservedTransferStaleNonceRejectedAfterExecution(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >= 7 {
			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	appendReservedTransferExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewReservedTransferExecution(
			proposal.ID,
		),
	)

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected first reserved transfer execution to pass consensus",
		)
	}

	chainID, err :=
		fixture.bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	// Same pool and nonce as the already-executed transfer,
	// but a different amount gives a distinct proposal ID.
	stale :=
		reserved.NewReservedTransferProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			fixture.sink.Address,
			101,
		)

	for _, authority := range fixture.authorities {

		if err :=
			stale.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	if stale.ID == proposal.ID {
		t.Fatal(
			"stale nonce test requires a distinct proposal ID",
		)
	}

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		stale,
	)

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected stale reserved transfer nonce to fail consensus",
		)
	}

	if _, err :=
		fixture.bc.GetGovernanceState(); err == nil {

		t.Fatal(
			"expected governance replay to reject stale reserved transfer nonce",
		)
	}
}

func TestReservedTransferCreditsOnlyAtExecution(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	before, err :=
		fixture.bc.BalanceOf(
			fixture.sink.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		signedReservedTransferConsensusProposal(
			t,
			fixture,
			1,
		)

	appendReservedTransferProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	afterProposal, err :=
		fixture.bc.BalanceOf(
			fixture.sink.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if afterProposal != before {
		t.Fatalf(
			"proposal credited recipient before execution: before=%d after=%d",
			before,
			afterProposal,
		)
	}

	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >= 7 {
			break
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}

	beforeExecution, err :=
		fixture.bc.BalanceOf(
			fixture.sink.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	appendReservedTransferExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewReservedTransferExecution(
			proposal.ID,
		),
	)

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected governed reserved transfer execution to pass consensus",
		)
	}

	afterExecution, err :=
		fixture.bc.BalanceOf(
			fixture.sink.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	expected :=
		beforeExecution + proposal.Amount

	if afterExecution != expected {
		t.Fatalf(
			"reserved transfer did not credit recipient exactly once: before=%d amount=%d after=%d expected=%d",
			beforeExecution,
			proposal.Amount,
			afterExecution,
			expected,
		)
	}

	accounting, err :=
		fixture.bc.GetReservedAccountingState()

	if err != nil {
		t.Fatal(err)
	}

	if accounting.Usage.Treasury != proposal.Amount {
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
			"executed reserved transfer remained pending",
		)
	}
}
