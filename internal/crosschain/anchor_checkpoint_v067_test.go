package crosschain

import (
	"strings"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func extendCheckpointFixtureV067(t *testing.T, f *anchorIncludedFixtureV067, successors int) {
	t.Helper()
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < successors; i++ {
		task, err := usefulwork.NewSumSquaresTask([]uint64{uint64(9000 + i)})
		if err != nil {
			t.Fatal(err)
		}
		proof, err := usefulwork.Execute(task, worker)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.chain.AddBlock(nil, []usefulwork.Proof{proof}, f.pos.Validators[0].Address, f.pos); err != nil {
			t.Fatal(err)
		}
	}
	if !f.chain.ValidateChain(f.pos) {
		t.Fatal("checkpoint fixture chain invalid")
	}
}

func newCheckpointFixtureV067(t *testing.T) (anchorIncludedFixtureV067, PinnedSourceCheckpointV067) {
	t.Helper()
	f := newAnchorIncludedFixtureV067(t)
	extendCheckpointFixtureV067(t, &f, 2)
	checkpoint := PinnedSourceCheckpointV067{
		SourceChainID:       f.candidate.policy.SourceChainID,
		SourceGenesisHash:   f.candidate.policy.SourceGenesisHash,
		CheckpointHeight:    f.chain.Blocks[2].Height,
		CheckpointBlockHash: f.chain.Blocks[2].Hash,
		MinSuccessorBlocks:  1,
	}
	return f, checkpoint
}

func (f anchorIncludedFixtureV067) checkCheckpoint(checkpoint PinnedSourceCheckpointV067) error {
	c := f.candidate
	return VerifyAnchorWithPinnedCheckpointV067(
		c.intent, c.job, c.contract, c.proof, c.report, c.policy,
		checkpoint, f.chain, f.pos,
	)
}

func TestAnchorPinnedCheckpointV067ValidLocalEvidence(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	if err := f.checkCheckpoint(cp); err != nil {
		t.Fatalf("valid local checkpoint evidence rejected: %v", err)
	}
	// A checkpoint pin need not be the current tip: it remains a LOCAL view.
	cp.CheckpointHeight = f.chain.Blocks[3].Height
	cp.CheckpointBlockHash = f.chain.Blocks[3].Hash
	cp.MinSuccessorBlocks = 2
	if err := f.checkCheckpoint(cp); err != nil {
		t.Fatalf("valid deeper local checkpoint evidence rejected: %v", err)
	}
}

func TestAnchorPinnedCheckpointV067RejectsAdversarialCases(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*anchorIncludedFixtureV067, *PinnedSourceCheckpointV067)
	}{
		{"wrong-checkpoint-chain", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) { c.SourceChainID = "prism-other" }},
		{"wrong-checkpoint-genesis", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) {
			c.SourceGenesisHash = strings.Repeat("a", 64)
		}},
		{"wrong-checkpoint-hash", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) {
			c.CheckpointBlockHash = strings.Repeat("f", 64)
		}},
		{"uppercase-checkpoint-hash", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) {
			c.CheckpointBlockHash = strings.Repeat("F", 64)
		}},
		{"zero-checkpoint-hash", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) {
			c.CheckpointBlockHash = strings.Repeat("0", 64)
		}},
		{"zero-depth", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) { c.MinSuccessorBlocks = 0 }},
		{"excessive-depth", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) { c.MinSuccessorBlocks = 129 }},
		{"insufficient-depth", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) { c.MinSuccessorBlocks = 2 }},
		{"checkpoint-at-inclusion", func(f *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) {
			c.CheckpointHeight = f.candidate.intent.SourceBlockHeight
			c.CheckpointBlockHash = f.candidate.intent.SourceBlockHash
		}},
		{"checkpoint-future-height", func(_ *anchorIncludedFixtureV067, c *PinnedSourceCheckpointV067) { c.CheckpointHeight = 1000 }},
		{"missing-checkpoint-block", func(f *anchorIncludedFixtureV067, _ *PinnedSourceCheckpointV067) { f.chain.Blocks = f.chain.Blocks[:2] }},
		{"mutated-local-checkpoint", func(f *anchorIncludedFixtureV067, _ *PinnedSourceCheckpointV067) {
			// Rewriting the pinned block and its descendants must not be accepted.
			f.chain.Blocks[2].Timestamp = f.chain.Blocks[2].Timestamp.Add(time.Second)
			f.chain.Blocks[2].Hash = blockchain.CalculateHash(f.chain.Blocks[2])
			f.chain.Blocks[3].PreviousHash = f.chain.Blocks[2].Hash
			f.chain.Blocks[3].Hash = blockchain.CalculateHash(f.chain.Blocks[3])
		}},
		{"wrong-source-inclusion", func(f *anchorIncludedFixtureV067, _ *PinnedSourceCheckpointV067) {
			f.candidate.intent.SourceBlockHeight++
		}},
		{"tampered-worker-proof", func(f *anchorIncludedFixtureV067, _ *PinnedSourceCheckpointV067) { f.candidate.proof.Signature = "00" }},
		{"nil-validator-set", func(f *anchorIncludedFixtureV067, _ *PinnedSourceCheckpointV067) { f.pos = nil }},
		{"nil-chain", func(f *anchorIncludedFixtureV067, _ *PinnedSourceCheckpointV067) { f.chain = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, cp := newCheckpointFixtureV067(t)
			tc.mutate(&f, &cp)
			if err := f.checkCheckpoint(cp); err == nil {
				t.Fatal("untrusted checkpoint or invalid local chain unexpectedly accepted")
			}
		})
	}
}
