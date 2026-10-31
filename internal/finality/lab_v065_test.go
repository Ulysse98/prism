package finality

import (
	"crypto/ed25519"
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

type quorumFixture struct {
	voters     []*wallet.Wallet
	validators []consensus.Validator
	ctx        VoteContext
}

func newQuorumFixture(t *testing.T, stakes ...uint64) quorumFixture {
	t.Helper()
	f := quorumFixture{}
	for _, stake := range stakes {
		signer, err := wallet.New()
		if err != nil {
			t.Fatal(err)
		}
		f.voters = append(f.voters, signer)
		f.validators = append(f.validators, consensus.Validator{Address: signer.Address, Stake: stake})
	}
	commitment, err := ValidatorSetHash(f.validators)
	if err != nil {
		t.Fatal(err)
	}
	f.ctx = VoteContext{
		ChainID:          "prism-test-finality-v065",
		GenesisHash:      strings.Repeat("a", 64),
		Height:           42,
		Round:            1,
		Step:             stepPrecommit,
		BlockHash:        strings.Repeat("b", 64),
		ValidatorSetHash: commitment,
	}
	return f
}

func (f quorumFixture) vote(t *testing.T, i int) SignedVote {
	t.Helper()
	v, err := SignVote(f.ctx, f.voters[i].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestLabV065WeightedQuorumAndSignatureBinding(t *testing.T) {
	f := newQuorumFixture(t, 25, 25, 25, 25)
	cert := Certificate{Context: f.ctx, Votes: []SignedVote{f.vote(t, 0), f.vote(t, 1), f.vote(t, 2)}}
	signed, total, err := VerifyQuorum(f.validators, cert)
	if err != nil || signed != 75 || total != 100 {
		t.Fatalf("75/100 quorum: %d/%d %v", signed, total, err)
	}
	if _, _, err := VerifyQuorum(f.validators, Certificate{Context: f.ctx, Votes: cert.Votes[:2]}); err == nil {
		t.Fatal("50/100 accepted")
	}

	tests := []struct {
		name   string
		mutate func(*Certificate)
	}{
		{"duplicate", func(c *Certificate) { c.Votes = append(c.Votes, c.Votes[0]) }},
		{"wrong_network", func(c *Certificate) { c.Context.ChainID = "other" }},
		{"wrong_block", func(c *Certificate) { c.Context.BlockHash = strings.Repeat("c", 64) }},
		{"wrong_round", func(c *Certificate) { c.Context.Round++ }},
		{"wrong_step", func(c *Certificate) { c.Context.Step = stepPrevote }},
		{"invalid_signature", func(c *Certificate) { c.Votes[0].Signature = hex.EncodeToString(make([]byte, ed25519.SignatureSize)) }},
		{"forged_address", func(c *Certificate) { c.Votes[0].Address = f.voters[3].Address }},
		{"modified_validator_set", func(c *Certificate) { c.Context.ValidatorSetHash = strings.Repeat("f", 64) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			altered := Certificate{Context: cert.Context, Votes: append([]SignedVote(nil), cert.Votes...)}
			tc.mutate(&altered)
			if _, _, err := VerifyQuorum(f.validators, altered); err == nil {
				t.Fatal("invalid certificate accepted")
			}
		})
	}
	unknown, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	fake, err := SignVote(f.ctx, unknown.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	bad := Certificate{Context: f.ctx, Votes: append(append([]SignedVote(nil), cert.Votes...), fake)}
	if _, _, err := VerifyQuorum(f.validators, bad); err == nil {
		t.Fatal("unknown signer accepted")
	}
}

func TestLabV065StrictTwoThirdsAndStakeSnapshot(t *testing.T) {
	f := newQuorumFixture(t, 1, 1, 1)
	two := Certificate{Context: f.ctx, Votes: []SignedVote{f.vote(t, 0), f.vote(t, 1)}}
	signed, total, err := VerifyQuorum(f.validators, two)
	if err == nil || signed != 2 || total != 3 {
		t.Fatalf("exactly 2/3 must fail: %d/%d %v", signed, total, err)
	}
	two.Votes = append(two.Votes, f.vote(t, 2))
	if _, _, err := VerifyQuorum(f.validators, two); err != nil {
		t.Fatal(err)
	}
	reversed := append([]consensus.Validator(nil), f.validators...)
	reversed[0], reversed[2] = reversed[2], reversed[0]
	a, err := ValidatorSetHash(f.validators)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ValidatorSetHash(reversed)
	if err != nil || a != b {
		t.Fatalf("validator commitment is order dependent: %q != %q %v", a, b, err)
	}
	modified := append([]consensus.Validator(nil), f.validators...)
	modified[0].Stake = 2
	if _, _, err := VerifyQuorum(modified, two); err == nil {
		t.Fatal("modified stake snapshot accepted")
	}
	duplicate := append([]consensus.Validator(nil), f.validators...)
	duplicate = append(duplicate, duplicate[0])
	if _, err := ValidatorSetHash(duplicate); err == nil {
		t.Fatal("duplicate validator accepted")
	}
	zero := append([]consensus.Validator(nil), f.validators...)
	zero[0].Stake = 0
	if _, err := ValidatorSetHash(zero); err == nil {
		t.Fatal("zero stake accepted")
	}
	overflow := append([]consensus.Validator(nil), f.validators...)
	overflow[0].Stake = math.MaxUint64
	if _, err := ValidatorSetHash(overflow); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestLabV065VoteContextValidation(t *testing.T) {
	f := newQuorumFixture(t, 100)
	cases := []struct {
		name   string
		change func(*VoteContext)
	}{
		{"missing_chain", func(c *VoteContext) { c.ChainID = "" }},
		{"genesis", func(c *VoteContext) { c.GenesisHash = "incorrect" }},
		{"block", func(c *VoteContext) { c.BlockHash = "not-a-hash" }},
		{"height_zero", func(c *VoteContext) { c.Height = 0 }},
		{"unknown_step", func(c *VoteContext) { c.Step = "finalized" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := f.ctx
			tt.change(&ctx)
			if _, err := SignVote(ctx, f.voters[0].PrivateKey); err == nil {
				t.Fatal("invalid context signed")
			}
		})
	}
	if _, err := SignVote(f.ctx, ed25519.PrivateKey([]byte{1, 2})); err == nil {
		t.Fatal("invalid private key size accepted")
	}
}

// This documents an intentional gap: certificate checks cannot stop signers
// from signing two different blocks without a durable double-sign lock.
func TestLabV065CompetingQuorumsExposeMissingVoteLock(t *testing.T) {
	f := newQuorumFixture(t, 25, 25, 25, 25)
	one := Certificate{Context: f.ctx, Votes: []SignedVote{f.vote(t, 0), f.vote(t, 1), f.vote(t, 2)}}
	competingContext := f.ctx
	competingContext.BlockHash = strings.Repeat("c", 64)
	other := Certificate{Context: competingContext}
	for _, i := range []int{1, 2, 3} {
		signed, err := SignVote(competingContext, f.voters[i].PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		other.Votes = append(other.Votes, signed)
	}
	if _, _, err := VerifyQuorum(f.validators, one); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyQuorum(f.validators, other); err != nil {
		t.Fatal(err)
	}
	t.Log("LAB LIMITATION: two conflicting quorum proofs can coexist when validators double-sign; durable locking is required")
}
