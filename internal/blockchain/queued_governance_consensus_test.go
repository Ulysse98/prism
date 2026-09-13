package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

type queuedGovernanceConsensusFixture struct {
	bc          *Blockchain
	pos         *consensus.ProofOfStake
	validator   *wallet.Wallet
	authorities []*wallet.Wallet
	sink        *wallet.Wallet
}

func newQueuedGovernanceConsensusFixture(
	t *testing.T,
) queuedGovernanceConsensusFixture {
	t.Helper()

	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	sink, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	bc, err := NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := bc.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	bc.Config = ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authorityA.Address,
				authorityB.Address,
			},
			TreasuryThreshold: 2,
		},
	}

	return queuedGovernanceConsensusFixture{
		bc:        bc,
		pos:       pos,
		validator: validator,
		authorities: []*wallet.Wallet{
			authorityA,
			authorityB,
		},
		sink: sink,
	}
}

func advanceToQueuedGovernanceActivation(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
) {
	t.Helper()

	targetHeight :=
		QueuedGovernanceActivationHeight - 1

	for {
		last :=
			fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

		if last.Height >= targetHeight {
			return
		}

		appendQueuedGovernanceFillerBlock(
			t,
			fixture,
		)
	}
}

func signedQueuedGovernanceProposal(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	nonce uint64,
	target string,
) reserved.AuthorityProposal {
	t.Helper()

	chainID, err :=
		fixture.bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		reserved.NewAuthorityProposal(
			chainID,
			nonce,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target,
		)

	for _, authority := range fixture.authorities {

		if err := proposal.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	return proposal
}

func appendAuthorityProposalConsensusBlock(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	proposal reserved.AuthorityProposal,
) {
	t.Helper()

	bc := fixture.bc
	pos := fixture.pos

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	proposer, err :=
		pos.SelectProposer(
			previous.Hash,
			nextHeight,
		)

	if err != nil {
		t.Fatal(err)
	}

	block := Block{
		Height: nextHeight,
		Timestamp: time.Unix(
			int64(nextHeight+1000),
			0,
		).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       consensus.DefaultProposerReward,
		AuthorityProposals: []reserved.AuthorityProposal{
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

func appendAuthorityExecutionConsensusBlock(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	execution reserved.AuthorityExecution,
) {
	t.Helper()

	bc := fixture.bc
	pos := fixture.pos

	previous :=
		bc.Blocks[len(bc.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	proposer, err :=
		pos.SelectProposer(
			previous.Hash,
			nextHeight,
		)

	if err != nil {
		t.Fatal(err)
	}

	block := Block{
		Height: nextHeight,
		Timestamp: time.Unix(
			int64(nextHeight+2000),
			0,
		).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       consensus.DefaultProposerReward,
		AuthorityExecutions: []reserved.AuthorityExecution{
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

func appendQueuedGovernanceFillerBlock(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
) {
	t.Helper()

	bc := fixture.bc

	nonce, err :=
		bc.NonceOf(
			fixture.validator.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	tx :=
		transaction.New(
			fixture.validator.Address,
			fixture.sink.Address,
			1,
			nonce,
			fixture.validator.PublicKeyHex(),
		)

	if err := tx.Sign(
		fixture.validator.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

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

	if _, err :=
		bc.AddBlock(
			[]transaction.Transaction{
				tx,
			},
			nil,
			proposer.Address,
			fixture.pos,
		); err != nil {

		t.Fatal(err)
	}
}

func TestQueuedGovernanceProposalParticipatesInConsensus(
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

	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected queued authority proposal to pass consensus",
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
			"consensus reconstruction lost pending proposal",
		)
	}

	expectedProposalHeight :=
		QueuedGovernanceActivationHeight

	if pending.ProposalHeight !=
		expectedProposalHeight {

		t.Fatalf(
			"unexpected proposal height: got=%d expected=%d",
			pending.ProposalHeight,
			expectedProposalHeight,
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
			"pending proposal activated authority before execution",
		)
	}
}

func TestQueuedGovernanceExecutionFailsConsensusBeforeDelay(
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

	// Proposal included at height 3.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	// ProposalHeight 3 + delay 5 means execution is allowed
	// starting at height 8.
	//
	// Advance only through height 6 so execution lands at height 7.
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

	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			proposal.ID,
		),
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height != 7 {
		t.Fatalf(
			"expected early execution at height 7, got %d",
			last.Height,
		)
	}

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected execution before governance delay to fail consensus",
		)
	}

	if _, err :=
		fixture.bc.GetGovernanceState(); err == nil {

		t.Fatal(
			"expected governance reconstruction to reject early execution",
		)
	}
}

func TestQueuedGovernanceExecutionPassesConsensusAtBoundary(
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

	// Proposal at height 3.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	// Advance through height 7.
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

	// Exact boundary:
	// 3 + DefaultGovernanceDelayBlocks(5) = height 8.
	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			proposal.ID,
		),
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height != 8 {
		t.Fatalf(
			"expected boundary execution at height 8, got %d",
			last.Height,
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected execution at governance delay boundary to pass consensus",
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
			"executed proposal remained pending after consensus reconstruction",
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
			"executed queued governance proposal did not activate authority",
		)
	}
}

func TestQueuedGovernanceUnknownExecutionFailsConsensus(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	// Next block is exactly the v0.29 activation boundary.
	appendAuthorityExecutionConsensusBlock(
		t,
		fixture,
		reserved.NewAuthorityExecution(
			"unknown-proposal",
		),
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected unknown execution at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			last.Height,
		)
	}

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected unknown queued governance execution to fail consensus",
		)
	}
}

func TestQueuedGovernanceBelowThresholdProposalFailsConsensus(
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

	chainID, err :=
		fixture.bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		reserved.NewAuthorityProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			target.Address,
		)

	// Only one approval for a 2-of-2 policy.
	authority :=
		fixture.authorities[0]

	if err := proposal.AddApproval(
		authority.Address,
		authority.PublicKeyHex(),
		authority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected below-threshold proposal at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			last.Height,
		)
	}

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected below-threshold queued proposal to fail consensus",
		)
	}
}
