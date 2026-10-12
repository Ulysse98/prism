package blockchain

import (
	"strings"
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func v061ReplayFixture(t *testing.T) (queuedGovernanceConsensusFixture, reserved.GovernanceBootstrap, Block) {
	t.Helper()
	f := newQueuedGovernanceConsensusFixture(t)
	for f.bc.Blocks[len(f.bc.Blocks)-1].Height < 5 {
		appendQueuedGovernanceFillerBlock(t, f)
	}
	id, err := f.bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	next := f.bc.Blocks[len(f.bc.Blocks)-1].Height + 1
	b, err := reserved.NewGovernanceBootstrap(
		id, f.bc.Blocks[0].Hash, consensus.ReservedPoolTeam,
		[]string{f.authorities[0].Address, f.authorities[1].Address}, 2, 1, next,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddApproval(f.validator); err != nil {
		t.Fatal(err)
	}
	last := f.bc.Blocks[len(f.bc.Blocks)-1]
	proposer, err := f.pos.SelectProposer(last.Hash, next)
	if err != nil {
		t.Fatal(err)
	}
	block := Block{
		Height:               next,
		Timestamp:            time.Unix(20000, 0).UTC(),
		PreviousHash:         last.Hash,
		Proposer:             proposer.Address,
		Reward:               consensus.DefaultProposerReward,
		GovernanceBootstraps: []reserved.GovernanceBootstrap{b},
	}
	block.Hash = CalculateHash(block)
	return f, b, block
}

func TestV061ConsensusReplayAcceptsSignedBootstrap(t *testing.T) {
	f, bootstrap, block := v061ReplayFixture(t)
	if err := f.bc.AppendValidatedBlock(block, f.pos); err != nil {
		t.Fatalf("signed bootstrap failed canonical validation: %v", err)
	}
	state, err := f.bc.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.CurrentPolicy.Team) != 2 || state.CurrentPolicy.TeamThreshold != 2 {
		t.Fatal("replay did not activate signed team governance")
	}
	if state.CurrentPolicy.Team[0] != bootstrap.Authorities[0] ||
		state.CurrentPolicy.Team[1] != bootstrap.Authorities[1] {
		t.Fatal("activated authorities differ from signed intent")
	}
	if _, err := f.bc.GetReservedAccountingState(); err != nil {
		t.Fatalf("reserved accounting diverged after bootstrap: %v", err)
	}
	if !f.bc.ValidateChain(f.pos) {
		t.Fatal("bootstrap made chain invalid")
	}
}

func TestV061ConsensusReplayRejectsInvalidBootstrapWithoutMutation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Block)
	}{
		{"wrong height in signature", func(b *Block) { b.GovernanceBootstraps[0].ActivationHeight++ }},
		{"wrong genesis", func(b *Block) { b.GovernanceBootstraps[0].GenesisHash = strings.Repeat("f", 64) }},
		{"missing approval", func(b *Block) { b.GovernanceBootstraps[0].Approvals = nil }},
		{"bad signature", func(b *Block) { b.GovernanceBootstraps[0].Approvals[0].Signature = "00" }},
		{"two bootstraps", func(b *Block) { b.GovernanceBootstraps = append(b.GovernanceBootstraps, b.GovernanceBootstraps[0]) }},
		{"mixed transaction", func(b *Block) {
			b.Transactions = append(b.Transactions, b.Transactions...)
			b.AuthorityExecutions = []reserved.AuthorityExecution{{ProposalID: "bad"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _, block := v061ReplayFixture(t)
			tc.mutate(&block)
			block.Hash = CalculateHash(block)
			lastHash := f.bc.Blocks[len(f.bc.Blocks)-1].Hash
			lastLen := len(f.bc.Blocks)
			if err := f.bc.AppendValidatedBlock(block, f.pos); err == nil {
				t.Fatal("invalid bootstrap passed canonical consensus")
			}
			if len(f.bc.Blocks) != lastLen || f.bc.Blocks[len(f.bc.Blocks)-1].Hash != lastHash {
				t.Fatal("rejected bootstrap mutated canonical chain")
			}
		})
	}
}

func TestV061ConsensusReplayRejectsSecondBootstrap(t *testing.T) {
	f, _, first := v061ReplayFixture(t)
	if err := f.bc.AppendValidatedBlock(first, f.pos); err != nil {
		t.Fatal(err)
	}
	id, err := f.bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	next := first.Height + 1
	second, err := reserved.NewGovernanceBootstrap(
		id, f.bc.Blocks[0].Hash, consensus.ReservedPoolTeam,
		[]string{f.authorities[0].Address, f.authorities[1].Address}, 2, 1, next,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.AddApproval(f.validator); err != nil {
		t.Fatal(err)
	}
	proposer, err := f.pos.SelectProposer(first.Hash, next)
	if err != nil {
		t.Fatal(err)
	}
	block := Block{
		Height: next, Timestamp: time.Unix(20001, 0).UTC(),
		PreviousHash: first.Hash, Proposer: proposer.Address,
		Reward:               consensus.DefaultProposerReward,
		GovernanceBootstraps: []reserved.GovernanceBootstrap{second},
	}
	block.Hash = CalculateHash(block)
	if err := f.bc.AppendValidatedBlock(block, f.pos); err == nil {
		t.Fatal("duplicate bootstrap was accepted")
	}
	if !f.bc.ValidateChain(f.pos) {
		t.Fatal("duplicate rejection damaged chain")
	}
}

func TestV061ConsensusReplayRejectsGenesisBootstrap(t *testing.T) {
	f, intent, _ := v061ReplayFixture(t)
	f.bc.Blocks[0].GovernanceBootstraps = []reserved.GovernanceBootstrap{intent}
	if _, _, err := f.bc.replayReservedTransferConsensusState(); err == nil {
		t.Fatal("genesis bootstrap passed deterministic replay")
	}
}

func TestV061ConsensusReplayRejectsBeforeActivation(t *testing.T) {
	f, _, block := v061ReplayFixture(t)
	// Move the signed block and its intent to height 5 on a separate chain.
	// This tests the replay boundary, not an invalid previous_hash shortcut.
	truncated := append([]Block(nil), f.bc.Blocks[:5]...)
	f.bc.Blocks = truncated
	block.Height = 5
	block.PreviousHash = truncated[len(truncated)-1].Hash
	block.GovernanceBootstraps[0].ActivationHeight = 5
	block.Hash = CalculateHash(block)
	state, err := reserved.NewGovernanceStateForChain(block.GovernanceBootstraps[0].ChainID, f.bc.Config.ReservedAuthorities)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.bc.applyGovernanceBootstrapBlockV061(block, state); err == nil {
		t.Fatal("bootstrap before activation height accepted")
	}
}
