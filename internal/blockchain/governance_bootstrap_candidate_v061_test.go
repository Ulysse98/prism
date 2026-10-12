package blockchain

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func newV061Candidate(t *testing.T) (queuedGovernanceConsensusFixture, reserved.GovernanceBootstrap) {
	t.Helper()
	fixture := newQueuedGovernanceConsensusFixture(t)
	for fixture.bc.Blocks[len(fixture.bc.Blocks)-1].Height < 5 {
		appendQueuedGovernanceFillerBlock(t, fixture)
	}
	chainID, err := fixture.bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := reserved.NewGovernanceBootstrap(
		chainID,
		fixture.bc.Blocks[0].Hash,
		consensus.ReservedPoolTeam,
		[]string{fixture.authorities[0].Address, fixture.authorities[1].Address},
		2,
		1,
		6,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.AddApproval(fixture.validator); err != nil {
		t.Fatal(err)
	}
	return fixture, bootstrap
}

func TestV061BootstrapCandidatePreflightReadOnly(t *testing.T) {
	fixture, bootstrap := newV061Candidate(t)
	beforeHeight := fixture.bc.Blocks[len(fixture.bc.Blocks)-1].Height
	beforeHash := fixture.bc.Blocks[len(fixture.bc.Blocks)-1].Hash
	beforeChainID, err := fixture.bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	if err := fixture.bc.ValidateBootstrapCandidateV061(bootstrap, fixture.pos); err != nil {
		t.Fatalf("valid candidate rejected: %v", err)
	}
	if fixture.bc.Blocks[len(fixture.bc.Blocks)-1].Height != beforeHeight ||
		fixture.bc.Blocks[len(fixture.bc.Blocks)-1].Hash != beforeHash {
		t.Fatal("preflight mutated the canonical blockchain")
	}
	afterChainID, err := fixture.bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	if beforeChainID != afterChainID {
		t.Fatal("preflight changed chain identity")
	}
	policy, err := fixture.bc.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.CurrentPolicy.Team) != 0 {
		t.Fatal("preflight activated governance on the chain")
	}
}

func TestV061BootstrapCandidateRejectsUntrustedContext(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*reserved.GovernanceBootstrap)
	}{
		{"wrong height", func(b *reserved.GovernanceBootstrap) { b.ActivationHeight++ }},
		{"wrong genesis", func(b *reserved.GovernanceBootstrap) { b.GenesisHash = strings.Repeat("0", 64) }},
		{"changed authority", func(b *reserved.GovernanceBootstrap) { b.Authorities[0] = "prism_" + strings.Repeat("1", 40) }},
		{"missing quorum", func(b *reserved.GovernanceBootstrap) { b.Approvals = nil }},
		{"tampered approval", func(b *reserved.GovernanceBootstrap) { b.Approvals[0].Signature = "00" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture, bootstrap := newV061Candidate(t)
			bootstrap.Authorities = append([]string(nil), bootstrap.Authorities...)
			bootstrap.Approvals = append([]reserved.Approval(nil), bootstrap.Approvals...)
			tc.mutate(&bootstrap)
			if err := fixture.bc.ValidateBootstrapCandidateV061(bootstrap, fixture.pos); err == nil {
				t.Fatal("invalid bootstrap candidate was accepted")
			}
			if !fixture.bc.ValidateChain(fixture.pos) {
				t.Fatal("candidate rejection mutated the canonical chain")
			}
		})
	}
}

func TestV061BootstrapCandidateRejectsWrongValidatorSnapshot(t *testing.T) {
	fixture, bootstrap := newV061Candidate(t)
	fake := &consensus.ProofOfStake{Validators: []consensus.Validator{{
		Address: fixture.validator.Address, Stake: 11,
	}}}
	if err := fixture.bc.ValidateBootstrapCandidateV061(bootstrap, fake); err == nil {
		t.Fatal("unmatched validator stake passed candidate validation")
	}
}

func TestV061BootstrapCandidateRejectsNilInputs(t *testing.T) {
	fixture, bootstrap := newV061Candidate(t)
	var nilChain *Blockchain
	if err := nilChain.ValidateBootstrapCandidateV061(bootstrap, fixture.pos); err == nil {
		t.Fatal("nil chain accepted")
	}
	if err := fixture.bc.ValidateBootstrapCandidateV061(bootstrap, nil); err == nil {
		t.Fatal("nil consensus state accepted")
	}
}
