package crosschain

import (
	"strings"
	"testing"
)

func validAnchorIntentV067() AnchorIntentV067 {
	return AnchorIntentV067{
		Version:             AnchorIntentVersionV067,
		SourceChainID:       "prism-da50ca9fa9c11da1",
		SourceGenesisHash:   strings.Repeat("a", 64),
		SourceBlockHeight:   42,
		SourceBlockHash:     strings.Repeat("b", 64),
		JobID:               strings.Repeat("c", 64),
		ProofHash:           strings.Repeat("d", 64),
		DestinationChainID:  "eip155:421614",
		DestinationRegistry: "0x1111111111111111111111111111111111111111",
		Nonce:               1,
	}
}

func TestAnchorIntentV067Deterministic(t *testing.T) {
	a := validAnchorIntentV067()
	first, err := a.Digest()
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("digest is not deterministic SHA-256: %s, %s", first, second)
	}
}

func TestAnchorIntentV067DomainSeparation(t *testing.T) {
	original := validAnchorIntentV067()
	base, err := original.Digest()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*AnchorIntentV067){
		"source chain":         func(a *AnchorIntentV067) { a.SourceChainID = "prism-other" },
		"genesis":              func(a *AnchorIntentV067) { a.SourceGenesisHash = strings.Repeat("e", 64) },
		"block height":         func(a *AnchorIntentV067) { a.SourceBlockHeight++ },
		"block hash":           func(a *AnchorIntentV067) { a.SourceBlockHash = strings.Repeat("e", 64) },
		"job":                  func(a *AnchorIntentV067) { a.JobID = strings.Repeat("e", 64) },
		"proof":                func(a *AnchorIntentV067) { a.ProofHash = strings.Repeat("e", 64) },
		"destination chain":    func(a *AnchorIntentV067) { a.DestinationChainID = "sui:testnet" },
		"destination registry": func(a *AnchorIntentV067) { a.DestinationRegistry = "0x2222222222222222222222222222222222222222" },
		"nonce":                func(a *AnchorIntentV067) { a.Nonce++ },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			altered := original
			change(&altered)
			changed, err := altered.Digest()
			if err != nil {
				t.Fatal(err)
			}
			if changed == base {
				t.Fatal("different intent has same digest")
			}
		})
	}
}

func TestAnchorIntentV067FailsClosed(t *testing.T) {
	cases := map[string]func(*AnchorIntentV067){
		"wrong version":       func(a *AnchorIntentV067) { a.Version = 2 },
		"bad source chain":    func(a *AnchorIntentV067) { a.SourceChainID = "PRISM other" },
		"missing genesis":     func(a *AnchorIntentV067) { a.SourceGenesisHash = "" },
		"uppercase genesis":   func(a *AnchorIntentV067) { a.SourceGenesisHash = strings.Repeat("A", 64) },
		"zero genesis":        func(a *AnchorIntentV067) { a.SourceGenesisHash = strings.Repeat("0", 64) },
		"zero height":         func(a *AnchorIntentV067) { a.SourceBlockHeight = 0 },
		"bad block hash":      func(a *AnchorIntentV067) { a.SourceBlockHash = strings.Repeat("z", 64) },
		"bad job":             func(a *AnchorIntentV067) { a.JobID = "job 123" },
		"zero proof hash":     func(a *AnchorIntentV067) { a.ProofHash = strings.Repeat("0", 64) },
		"bad destination":     func(a *AnchorIntentV067) { a.DestinationChainID = "eip155:421614 other" },
		"missing registry":    func(a *AnchorIntentV067) { a.DestinationRegistry = "" },
		"registry with slash": func(a *AnchorIntentV067) { a.DestinationRegistry = "https://example.org" },
		"zero nonce":          func(a *AnchorIntentV067) { a.Nonce = 0 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			a := validAnchorIntentV067()
			change(&a)
			if err := a.Validate(); err == nil {
				t.Fatal("accepted invalid intent")
			}
			if digest, err := a.Digest(); err == nil || digest != "" {
				t.Fatalf("Digest() must fail closed, got %q, %v", digest, err)
			}
		})
	}
}
