package reserved

import (
	"crypto/ed25519"
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

const testBootstrapGenesis = "807567a62284fb7990ab6a3d75c0a8255928a5d3b8681ec7fb86eac8615c5f85"
const testBootstrapChain = "prism-ceea53aa87bcae01"

func bootstrapFixture(t *testing.T) (GovernanceBootstrap, []consensus.Validator, []*wallet.Wallet) {
	t.Helper()
	wallets := make([]*wallet.Wallet, 3)
	vals := make([]consensus.Validator, 3)
	amounts := []uint64{500, 300, 200}
	for i := range wallets {
		w, err := wallet.New()
		if err != nil {
			t.Fatal(err)
		}
		wallets[i] = w
		vals[i] = consensus.Validator{Address: w.Address, Stake: amounts[i]}
	}
	b, err := NewGovernanceBootstrap(testBootstrapChain, testBootstrapGenesis, consensus.ReservedPoolTeam, []string{wallets[0].Address, wallets[1].Address, wallets[2].Address}, 2, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	return b, vals, wallets
}

func TestV061BootstrapQuorumAndDomain(t *testing.T) {
	b, vals, w := bootstrapFixture(t)
	if err := b.AddApproval(w[0]); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGovernanceBootstrap(b, vals, testBootstrapChain, testBootstrapGenesis, 4, 1); err == nil {
		t.Fatal("500 stake must not pass a >2/3 quorum")
	}
	if err := b.AddApproval(w[1]); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGovernanceBootstrap(b, vals, testBootstrapChain, testBootstrapGenesis, 4, 1); err != nil {
		t.Fatal(err)
	}
	if err := b.AddApproval(w[1]); err == nil {
		t.Fatal("duplicate approval accepted")
	}

	tests := []struct {
		name    string
		change  func(*GovernanceBootstrap)
		chain   string
		genesis string
		height  uint64
		nonce   uint64
	}{
		{name: "other chain", chain: "prism-other", genesis: testBootstrapGenesis, height: 4, nonce: 1},
		{name: "other genesis", chain: testBootstrapChain, genesis: strings.Repeat("f", 64), height: 4, nonce: 1},
		{name: "wrong activation", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 5, nonce: 1},
		{name: "replay nonce", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 4, nonce: 2},
		{name: "tampered threshold", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 4, nonce: 1, change: func(b *GovernanceBootstrap) { b.Threshold = 3 }},
		{name: "tampered authority", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 4, nonce: 1, change: func(b *GovernanceBootstrap) { b.Authorities[0] = "prism_" + strings.Repeat("0", 40) }},
		{name: "tampered id", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 4, nonce: 1, change: func(b *GovernanceBootstrap) { b.ID = strings.Repeat("0", 64) }},
		{name: "tampered signature", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 4, nonce: 1, change: func(b *GovernanceBootstrap) {
			b.Approvals[0].Signature = hex.EncodeToString(make([]byte, ed25519.SignatureSize))
		}},
		{name: "duplicate signer", chain: testBootstrapChain, genesis: testBootstrapGenesis, height: 4, nonce: 1, change: func(b *GovernanceBootstrap) { b.Approvals[1] = b.Approvals[0] }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clone := b
			clone.Approvals = append([]Approval(nil), b.Approvals...)
			clone.Authorities = append([]string(nil), b.Authorities...)
			if tc.change != nil {
				tc.change(&clone)
			}
			if err := ValidateGovernanceBootstrap(clone, vals, tc.chain, tc.genesis, tc.height, tc.nonce); err == nil {
				t.Fatal("unsafe bootstrap accepted")
			}
		})
	}
}

func TestV061BootstrapValidatorMembershipAndThreshold(t *testing.T) {
	b, vals, w := bootstrapFixture(t)
	outsider, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddApproval(w[0]); err != nil {
		t.Fatal(err)
	}
	if err := b.AddApproval(outsider); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGovernanceBootstrap(b, vals, testBootstrapChain, testBootstrapGenesis, 4, 1); err == nil {
		t.Fatal("outsider approval accepted")
	}

	b, vals, w = bootstrapFixture(t)
	_ = b.AddApproval(w[0])
	_ = b.AddApproval(w[2]) // 700/1000 > 2/3
	if err := ValidateGovernanceBootstrap(b, vals, testBootstrapChain, testBootstrapGenesis, 4, 1); err != nil {
		t.Fatal(err)
	}
	doubled := append(append([]consensus.Validator(nil), vals...), vals[0])
	if err := ValidateGovernanceBootstrap(b, doubled, testBootstrapChain, testBootstrapGenesis, 4, 1); err == nil {
		t.Fatal("duplicate validator accepted")
	}
	overflow := []consensus.Validator{{Address: vals[0].Address, Stake: math.MaxUint64}, {Address: vals[1].Address, Stake: 1}}
	if err := ValidateGovernanceBootstrap(b, overflow, testBootstrapChain, testBootstrapGenesis, 4, 1); err == nil {
		t.Fatal("stake overflow accepted")
	}
}

func TestV061BootstrapStrictTwoThirds(t *testing.T) {
	b, vals, w := bootstrapFixture(t)
	// Stake 3+2+1 = 6; signatures from 3+1 represent exactly 2/3.
	vals[0].Stake = 3
	vals[1].Stake = 2
	vals[2].Stake = 1
	_ = b.AddApproval(w[0])
	_ = b.AddApproval(w[2]) // 4 of 6: exactly two-thirds
	if err := ValidateGovernanceBootstrap(b, vals, testBootstrapChain, testBootstrapGenesis, 4, 1); err == nil {
		t.Fatal("exact two-thirds passed")
	}
}

func TestV061BootstrapRejectsUnsafeAuthorityPolicy(t *testing.T) {
	_, _, w := bootstrapFixture(t)
	for _, tc := range []struct {
		name      string
		threshold uint32
		addresses []string
	}{
		{"single signer", 1, []string{w[0].Address, w[1].Address}},
		{"too high", 3, []string{w[0].Address, w[1].Address}},
		{"duplicate", 2, []string{w[0].Address, w[0].Address}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewGovernanceBootstrap(testBootstrapChain, testBootstrapGenesis, consensus.ReservedPoolTeam, tc.addresses, tc.threshold, 1, 4); err == nil {
				t.Fatal("unsafe authority policy accepted")
			}
		})
	}
}
