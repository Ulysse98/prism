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
