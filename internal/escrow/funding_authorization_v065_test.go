package escrow

import (
	"strings"
	"testing"

	"prism/internal/wallet"
)

func TestFundingAuthorizationV065BindsAllFields(t *testing.T) {
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	f := Funding{ChainID: "prism-" + strings.Repeat("a", 16), GenesisHash: strings.Repeat("b", 64), JobID: strings.Repeat("c", 64), Requester: alice.Address, Amount: 10, FundedAtHeight: 5, ExpiresAtHeight: 15}
	signed, err := SignFunding(f, 7, alice.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyFunding(signed); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*FundingAuthorization)
	}{
		{"amount", func(a *FundingAuthorization) { a.Funding.Amount++ }},
		{"height", func(a *FundingAuthorization) { a.Funding.FundedAtHeight++ }},
		{"expiry", func(a *FundingAuthorization) { a.Funding.ExpiresAtHeight++ }},
		{"job", func(a *FundingAuthorization) { a.Funding.JobID = strings.Repeat("1", 64) }},
		{"chain", func(a *FundingAuthorization) { a.Funding.ChainID = "prism-" + strings.Repeat("1", 16) }},
		{"genesis", func(a *FundingAuthorization) { a.Funding.GenesisHash = strings.Repeat("1", 64) }},
		{"requester", func(a *FundingAuthorization) { a.Funding.Requester = "prism_" + strings.Repeat("1", 40) }},
		{"nonce", func(a *FundingAuthorization) { a.Nonce++ }},
		{"key", func(a *FundingAuthorization) { a.PublicKey = strings.Repeat("1", 64) }},
		{"signature", func(a *FundingAuthorization) { a.Signature = strings.Repeat("1", 128) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := signed
			c.mutate(&bad)
			if err := VerifyFunding(bad); err == nil {
				t.Fatal("tampered funding accepted")
			}
		})
	}
}

func TestFundingAuthorizationV065CannotUseWrongKey(t *testing.T) {
	alice, _ := wallet.New()
	bob, _ := wallet.New()
	f := Funding{ChainID: "prism-" + strings.Repeat("a", 16), GenesisHash: strings.Repeat("b", 64), JobID: strings.Repeat("c", 64), Requester: alice.Address, Amount: 10, FundedAtHeight: 5, ExpiresAtHeight: 15}
	if _, err := SignFunding(f, 0, bob.PrivateKey); err == nil {
		t.Fatal("wrong key accepted")
	}
}
