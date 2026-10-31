package p2p

import (
	"bytes"
	"encoding/json"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/finality"
)

func TestEscrowV065CandidateForFinalityVote(t *testing.T) {
	base, pos, alice, _, proof, _ := escrowCompetingFixtureV065(t, 10)
	full := cloneEscrowTipV065(base)
	proposed, err := full.AddEscrowReleaseBlockV065(proof, alice.Address, pos)
	if err != nil {
		t.Fatal(err)
	}

	chainID, _, err := networkIdentity(base)
	if err != nil {
		t.Fatal(err)
	}
	setHash, err := finality.ValidatorSetHash(pos.Validators)
	if err != nil {
		t.Fatal(err)
	}
	trustedTip := base.Blocks[len(base.Blocks)-1].Hash
	ctx := finality.VoteContext{
		ChainID: chainID, GenesisHash: base.Blocks[0].Hash,
		Height: proposed.Height, Round: 0, Step: "prevote",
		BlockHash: proposed.Hash, ValidatorSetHash: setHash,
	}
	check := func(b *blockchain.Blockchain, c blockchain.Block, v finality.VoteContext, cid, tip string) error {
		return finality.ValidatePrismProposalV065(b, pos, c, v, cid, tip)
	}

	before, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := check(base, proposed, ctx, chainID, trustedTip); err != nil {
		t.Fatalf("valid RELEASE rejected: %v", err)
	}
	if err := check(base, proposed, ctx, chainID, trustedTip); err != nil {
		t.Fatalf("repeat valid check rejected: %v", err)
	}
	if after, err := json.Marshal(base); err != nil || !bytes.Equal(before, after) {
		t.Fatal("validation changed original parent chain")
	}

	cases := []struct {
		name string
		edit func(*blockchain.Block, *finality.VoteContext)
	}{
		{"wrong height", func(b *blockchain.Block, _ *finality.VoteContext) { b.Height++ }},
		{"wrong previous hash", func(b *blockchain.Block, _ *finality.VoteContext) { b.PreviousHash = ctx.GenesisHash }},
		{"wrong block hash", func(b *blockchain.Block, _ *finality.VoteContext) { b.Hash = ctx.GenesisHash }},
		{"wrong proposer with recomputed hash", func(b *blockchain.Block, v *finality.VoteContext) {
			b.Proposer = "prism_unregistered_proposer"
			b.Hash = blockchain.CalculateHash(*b)
			v.BlockHash = b.Hash
		}},
		{"removed escrow proof with recomputed hash", func(b *blockchain.Block, v *finality.VoteContext) {
			b.EscrowReleases = nil
			b.Hash = blockchain.CalculateHash(*b)
			v.BlockHash = b.Hash
		}},
		{"different trusted vote block", func(_ *blockchain.Block, v *finality.VoteContext) { v.BlockHash = ctx.GenesisHash }},
		{"different genesis", func(_ *blockchain.Block, v *finality.VoteContext) { v.GenesisHash = ctx.BlockHash }},
		{"different validator set", func(_ *blockchain.Block, v *finality.VoteContext) { v.ValidatorSetHash = ctx.GenesisHash }},
		{"NIL vote", func(_ *blockchain.Block, v *finality.VoteContext) { v.Nil = true; v.BlockHash = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate, vote := proposed, ctx
			tc.edit(&candidate, &vote)
			if err := check(base, candidate, vote, chainID, trustedTip); err == nil {
				t.Fatal("unsafe candidate was accepted")
			}
		})
	}
	if err := check(base, proposed, ctx, "different-network", trustedTip); err == nil {
		t.Fatal("wrong network was accepted")
	}
	if err := check(base, proposed, ctx, chainID, proposed.Hash); err == nil {
		t.Fatal("untrusted parent was accepted")
	}
	t.Log("valid PRISM escrow candidate accepted; corrupted candidate/context rejected without mutation")
}
