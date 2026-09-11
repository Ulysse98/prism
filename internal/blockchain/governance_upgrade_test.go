package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func signedDirectAuthorityChange(
	t *testing.T,
	fixture queuedGovernanceConsensusFixture,
	nonce uint64,
	target string,
) reserved.AuthorityChange {
	t.Helper()

	chainID, err := fixture.bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	change := reserved.NewAuthorityChange(
		chainID,
		nonce,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		target,
	)

	for _, authority := range fixture.authorities {
		if err := change.AddApproval(
			authority.Address,
			authority.PublicKeyHex(),
			authority.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}
	}

	return change
}

func TestDirectAuthorityChangeAllowedBeforeQueuedGovernanceActivation(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	// Height 1 remains historical.
	appendQueuedGovernanceFillerBlock(
		t,
		fixture,
	)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		signedDirectAuthorityChange(
			t,
			fixture,
			1,
			target.Address,
		)

	// Direct AuthorityChange lands at height 2, the final
	// historical governance height.
	appendGovernanceConsensusBlock(
		t,
		fixture.bc,
		fixture.pos,
		change,
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	expectedHeight :=
		QueuedGovernanceActivationHeight - 1

	if last.Height != expectedHeight {
		t.Fatalf(
			"expected historical direct authority change at height %d, got %d",
			expectedHeight,
			last.Height,
		)
	}

	if !fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected direct authority change before queued governance activation to remain consensus-valid",
		)
	}

	state, err :=
		fixture.bc.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
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
			"historical direct authority change was not applied",
		)
	}
}

func TestDirectAuthorityChangeRejectedAtQueuedGovernanceActivation(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	// Bring the chain through historical height 2.
	advanceToQueuedGovernanceActivation(
		t,
		fixture,
	)

	target, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	change :=
		signedDirectAuthorityChange(
			t,
			fixture,
			1,
			target.Address,
		)

	// This direct AuthorityChange lands exactly at height 3.
	// It is otherwise fully signed and valid, so the only reason
	// for rejection must be the v0.29 governance boundary.
	appendGovernanceConsensusBlock(
		t,
		fixture.bc,
		fixture.pos,
		change,
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	if last.Height !=
		QueuedGovernanceActivationHeight {

		t.Fatalf(
			"expected direct authority change at activation height %d, got %d",
			QueuedGovernanceActivationHeight,
			last.Height,
		)
	}

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected direct authority change at queued governance activation to fail consensus",
		)
	}

	if _, err :=
		fixture.bc.GetGovernanceState(); err == nil {

		t.Fatal(
			"expected governance reconstruction to reject direct authority change at queued governance activation",
		)
	}
}

func TestQueuedGovernanceProposalRejectedBeforeActivation(
	t *testing.T,
) {
	fixture :=
		newQueuedGovernanceConsensusFixture(t)

	// Advance only to height 1.
	appendQueuedGovernanceFillerBlock(
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

	// Proposal lands at height 2, one block before activation.
	appendAuthorityProposalConsensusBlock(
		t,
		fixture,
		proposal,
	)

	last :=
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1]

	expectedHeight :=
		QueuedGovernanceActivationHeight - 1

	if last.Height != expectedHeight {
		t.Fatalf(
			"expected pre-activation proposal at height %d, got %d",
			expectedHeight,
			last.Height,
		)
	}

	if fixture.bc.ValidateChain(
		fixture.pos,
	) {
		t.Fatal(
			"expected queued governance proposal before activation to fail consensus",
		)
	}

	if _, err :=
		fixture.bc.GetGovernanceState(); err == nil {

		t.Fatal(
			"expected governance reconstruction to reject queued proposal before activation",
		)
	}
}
