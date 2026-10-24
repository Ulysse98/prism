//go:build linux

package p2p

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/finality"
)

// This integration fixture uses real escrow blocks and the canonical chain
// validator; it does not run against mocked blockchain validation.
func TestEscrowV065ValidatedPrevoteBridge(t *testing.T) {
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
	genesis := base.Blocks[0].Hash
	tip := base.Blocks[len(base.Blocks)-1].Hash
	const storageFile = "bft-safety-v065.json"
	init := func(dir string) {
		t.Helper()
		if err := finality.InitPrismValidatedPrevoteGateV065(dir, chainID, genesis, proposed.Height, pos.Validators, alice.PrivateKey, base, tip); err != nil {
			t.Fatal(err)
		}
	}
	sign := func(dir string, candidate blockchain.Block) (finality.SignedVote, error) {
		return finality.SignPrismValidatedPrevoteV065(dir, chainID, genesis, proposed.Height, pos.Validators, alice.PrivateKey, candidate, tip)
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	init(dir)
	vote, err := sign(dir, proposed)
	if err != nil || vote.Context.BlockHash != proposed.Hash || vote.Context.Nil {
		t.Fatalf("validated escrow PREVOTE: %v", err)
	}
	retry, err := sign(dir, proposed)
	if err != nil || retry != vote {
		t.Fatalf("durable retry differs from recorded signature: %v", err)
	}
	if _, err := finality.SignDurableGatePrevoteV065(dir, chainID, genesis, proposed.Height, pos.Validators, alice.PrivateKey, proposed.Hash, true, nil); err == nil {
		t.Fatal("unprotected bool-only gate bypassed authenticated mode")
	}
	if _, err := finality.SignDurableGatePrecommitV065(dir, chainID, genesis, proposed.Height, pos.Validators, alice.PrivateKey, nil, true); err == nil {
		t.Fatal("unprotected PRECOMMIT bypassed authenticated mode")
	}
	// Corrupt the persisted candidate witness without changing the signed vote.
	path := filepath.Join(dir, storageFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	events, ok := state["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatal("stored candidate event missing")
	}
	event, ok := events[0].(map[string]any)
	if !ok {
		t.Fatal("bad event shape")
	}
	candidate, ok := event["candidate"].(map[string]any)
	if !ok {
		t.Fatal("candidate witness missing")
	}
	candidate["hash"] = tip
	tampered, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := sign(dir, proposed); err == nil {
		t.Fatal("tampered witness passed replay")
	}

	// Fresh gate must reject tampered escrow data *before* returning any vote.
	clean := t.TempDir()
	if err := os.Chmod(clean, 0700); err != nil {
		t.Fatal(err)
	}
	init(clean)
	invalid := proposed
	invalid.EscrowReleases = nil
	invalid.Hash = blockchain.CalculateHash(invalid)
	if _, err := sign(clean, invalid); err == nil {
		t.Fatal("invalid escrow candidate signed")
	}
	first, err := os.ReadFile(filepath.Join(clean, storageFile))
	if err != nil {
		t.Fatal(err)
	}
	var unchanged struct {
		Revision uint64                `json:"revision"`
		Votes    []finality.SignedVote `json:"votes"`
	}
	if err := json.Unmarshal(first, &unchanged); err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != 1 || len(unchanged.Votes) != 0 {
		t.Fatal("rejected candidate left durable vote")
	}
	if _, err := sign(clean, proposed); err != nil {
		t.Fatalf("valid block rejected following failed candidate: %v", err)
	}
}
