package reserved

import (
	"reflect"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

func approvedBootstrapFixture(t *testing.T) (GovernanceBootstrap, []consensus.Validator, []*wallet.Wallet) {
	t.Helper()
	b, vals, wallets := bootstrapFixture(t)
	for _, signer := range wallets[:2] { // 500 + 300 of 1000 stake
		if err := b.AddApproval(signer); err != nil {
			t.Fatal(err)
		}
	}
	return b, vals, wallets
}

func TestV061BootstrapStateAuthorizedTransition(t *testing.T) {
	b, vals, wallets := approvedBootstrapFixture(t)
	state, err := NewGovernanceStateForChain(testBootstrapChain, AuthorityPolicy{
		Treasury:          []string{wallets[2].Address},
		TreasuryThreshold: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	treasuryBefore := append([]string(nil), state.CurrentPolicy.Treasury...)
	if err := state.ApplyBootstrapV061(b, vals, testBootstrapGenesis, 4); err != nil {
		t.Fatalf("valid bootstrap rejected: %v", err)
	}
	if !reflect.DeepEqual(state.CurrentPolicy.Team, b.Authorities) {
		t.Fatal("bootstrap did not install canonical team authorities")
	}
	if state.CurrentPolicy.TeamThreshold != 2 {
		t.Fatal("wrong team threshold")
	}
	if !reflect.DeepEqual(state.CurrentPolicy.Treasury, treasuryBefore) || state.CurrentPolicy.TreasuryThreshold != 1 {
		t.Fatal("bootstrap unexpectedly modified treasury policy")
	}
	if err := state.ApplyBootstrapV061(b, vals, testBootstrapGenesis, 4); err == nil {
		t.Fatal("second bootstrap unexpectedly succeeded")
	}
}

func TestV061BootstrapStateRejectsInvalidAuthorizationAtomically(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*GovernanceBootstrap, *[]consensus.Validator, *uint64, *string)
	}{
		{"bad block height", func(_ *GovernanceBootstrap, _ *[]consensus.Validator, h *uint64, _ *string) { *h = 5 }},
		{"wrong genesis", func(_ *GovernanceBootstrap, _ *[]consensus.Validator, _ *uint64, g *string) {
			*g = strings.Repeat("f", 64)
		}},
		{"unexpected nonce", func(b *GovernanceBootstrap, _ *[]consensus.Validator, _ *uint64, _ *string) { b.Nonce = 2 }},
		{"removed approval", func(b *GovernanceBootstrap, _ *[]consensus.Validator, _ *uint64, _ *string) {
			b.Approvals = b.Approvals[:1]
		}},
		{"altered signature", func(b *GovernanceBootstrap, _ *[]consensus.Validator, _ *uint64, _ *string) {
			b.Approvals[0].Signature = "00"
		}},
		{"wrong stake", func(_ *GovernanceBootstrap, validators *[]consensus.Validator, _ *uint64, _ *string) {
			(*validators)[0].Stake = 1
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, vals, _ := approvedBootstrapFixture(t)
			b.Authorities = append([]string(nil), b.Authorities...)
			b.Approvals = append([]Approval(nil), b.Approvals...)
			vals = append([]consensus.Validator(nil), vals...)
			height := uint64(4)
			genesis := testBootstrapGenesis
			tc.mutate(&b, &vals, &height, &genesis)
			state, err := NewGovernanceStateForChain(testBootstrapChain, AuthorityPolicy{})
			if err != nil {
				t.Fatal(err)
			}
			before := cloneAuthorityPolicy(state.CurrentPolicy)
			if err := state.ApplyBootstrapV061(b, vals, genesis, height); err == nil {
				t.Fatal("invalid bootstrap unexpectedly succeeded")
			}
			if !reflect.DeepEqual(state.CurrentPolicy, before) {
				t.Fatal("rejected bootstrap changed governance state")
			}
		})
	}
}

func TestV061BootstrapStateRequiresEmptyTeam(t *testing.T) {
	b, vals, wallets := approvedBootstrapFixture(t)
	state, err := NewGovernanceStateForChain(testBootstrapChain, AuthorityPolicy{
		Team: []string{wallets[0].Address}, TeamThreshold: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := cloneAuthorityPolicy(state.CurrentPolicy)
	if err := state.ApplyBootstrapV061(b, vals, testBootstrapGenesis, 4); err == nil {
		t.Fatal("non-empty team policy was replaced")
	}
	if !reflect.DeepEqual(state.CurrentPolicy, before) {
		t.Fatal("bootstrap mutated existing authorities")
	}
}

func TestV061BootstrapStateRejectsNilAndUnboundNetwork(t *testing.T) {
	b, vals, _ := approvedBootstrapFixture(t)
	var nilState *GovernanceState
	if err := nilState.ApplyBootstrapV061(b, vals, testBootstrapGenesis, 4); err == nil {
		t.Fatal("nil state accepted")
	}
	state, err := NewGovernanceState(AuthorityPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyBootstrapV061(b, vals, testBootstrapGenesis, 4); err == nil {
		t.Fatal("unbound chain state accepted")
	}
}
