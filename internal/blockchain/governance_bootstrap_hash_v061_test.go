package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func TestV061BlockHashCommitsBootstrapAndPreservesHistoricalDomain(t *testing.T) {
	historic := Block{
		Height:       7,
		Timestamp:    time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC),
		PreviousHash: "previous",
		Proposer:     "prism_historical_validator",
		Reward:       consensus.DefaultProposerReward,
	}
	legacyHash := CalculateHash(historic)
	if legacyHash == "" {
		t.Fatal("missing historical hash")
	}

	withBootstrap := historic
	withBootstrap.GovernanceBootstraps = []reserved.GovernanceBootstrap{{
		ID:               "demo-bootstrap",
		ChainID:          "prism-demo",
		GenesisHash:      "demo-genesis",
		Pool:             consensus.ReservedPoolTeam,
		Threshold:        2,
		Nonce:            1,
		ActivationHeight: 7,
		Authorities:      []string{"authority-a", "authority-b"},
		Approvals:        []reserved.Approval{{Authorizer: "validator-a", Signature: "signature-a"}},
	}}
	signedHash := CalculateHash(withBootstrap)
	if signedHash == legacyHash {
		t.Fatal("bootstrap used the historical hash domain")
	}
	changedAuthority := withBootstrap
	b := withBootstrap.GovernanceBootstraps[0]
	b.Authorities = append([]string(nil), b.Authorities...)
	b.Authorities[0] = "modified-authority"
	changedAuthority.GovernanceBootstraps = []reserved.GovernanceBootstrap{b}
	if CalculateHash(changedAuthority) == signedHash {
		t.Fatal("bootstrap authority set missing from hash")
	}
	changedApproval := withBootstrap
	b = withBootstrap.GovernanceBootstraps[0]
	b.Approvals = append([]reserved.Approval(nil), b.Approvals...)
	b.Approvals[0].Signature = "signature-b"
	changedApproval.GovernanceBootstraps = []reserved.GovernanceBootstrap{b}
	if CalculateHash(changedApproval) == signedHash {
		t.Fatal("bootstrap approvals missing from hash")
	}
	if CalculateHash(historic) != legacyHash {
		t.Fatal("v0.61 changed historical hash computation")
	}
}

func TestV061BootstrapBlocksFailClosedBeforeConsensusActivation(t *testing.T) {
	fixture := newQueuedGovernanceConsensusFixture(t)
	appendQueuedGovernanceFillerBlock(t, fixture)
	if !fixture.bc.ValidateChain(fixture.pos) {
		t.Fatal("baseline chain is invalid")
	}

	index := len(fixture.bc.Blocks) - 1
	block := fixture.bc.Blocks[index]
	block.GovernanceBootstraps = []reserved.GovernanceBootstrap{{ID: "unactivated"}}
	block.Hash = CalculateHash(block)
	fixture.bc.Blocks[index] = block

	if fixture.bc.ValidateChain(fixture.pos) {
		t.Fatal("an unactivated governance bootstrap passed chain validation")
	}
	if _, err := fixture.bc.GetState(); err == nil {
		t.Fatal("unactivated governance bootstrap passed state reconstruction")
	}
}
