package governancev069

import (
    "strings"
    "testing"
)

func validIntent() ProposalIntent {
    return ProposalIntent{
        Version: ProtocolVersion,
        Network: "prism-devnet",
        ChainID: "prism-test-01",
        GenesisSHA256: strings.Repeat("a", 64),
        Proposer: "prism1exampleproposer",
        Nonce: 42,
        CreatedAtHeight: 100,
        ExpiresAtHeight: 200,
        Topic: TopicParameterReview,
        Title: "Parameter discussion",
        Rationale: "This is a strictly advisory research proposal.",
        PayloadSHA256: strings.Repeat("b", 64),
    }
}

func validContext() Context {
    p := validIntent()
    return Context{Network: p.Network, ChainID: p.ChainID, GenesisSHA256: p.GenesisSHA256, CurrentHeight: 150}
}

func TestProposalIntentValidAndDeterministic(t *testing.T) {
    p := validIntent()
    if err := ValidateForContext(p, validContext()); err != nil { t.Fatal(err) }
    a, err := Commitment(p)
    if err != nil { t.Fatal(err) }
    b, err := Commitment(p)
    if err != nil { t.Fatal(err) }
    if a != b { t.Fatal("commitment not deterministic") }
    if a == [32]byte{} { t.Fatal("zero commitment") }
    gotHex, err := HexCommitment(p)
    if err != nil { t.Fatal(err) }
    const wantHex = "f0065c50f7ea73077d6f6cbde864e89e5761c57cc708e73b8531f7a6fecd5f19"
    if gotHex != wantHex { t.Fatalf("canonical vector changed: got %s, want %s", gotHex, wantHex) }
    k, err := ReplayKey(p)
    if err != nil { t.Fatal(err) }
    if k == a { t.Fatal("missing domain separation") }
}

func TestProposalIntentAllCommittedFieldsBound(t *testing.T) {
    original := validIntent()
    base, err := Commitment(original)
    if err != nil { t.Fatal(err) }
    changes := map[string]func(*ProposalIntent){
        "network": func(p *ProposalIntent) { p.Network += "-alt" },
        "chain": func(p *ProposalIntent) { p.ChainID += "-alt" },
        "genesis": func(p *ProposalIntent) { p.GenesisSHA256 = strings.Repeat("c", 64) },
        "proposer": func(p *ProposalIntent) { p.Proposer += "-other" },
        "nonce": func(p *ProposalIntent) { p.Nonce++ },
        "created": func(p *ProposalIntent) { p.CreatedAtHeight++ },
        "expires": func(p *ProposalIntent) { p.ExpiresAtHeight++ },
        "topic": func(p *ProposalIntent) { p.Topic = TopicUpgradeReview },
        "title": func(p *ProposalIntent) { p.Title += " edited" },
        "rationale": func(p *ProposalIntent) { p.Rationale += " Altered." },
        "payload": func(p *ProposalIntent) { p.PayloadSHA256 = strings.Repeat("c", 64) },
    }
    for name, alter := range changes {
        t.Run(name, func(t *testing.T) {
            p := original
            alter(&p)
            got, err := Commitment(p)
            if err != nil { t.Fatal(err) }
            if got == base { t.Fatal("tampered field did not change commitment") }
        })
    }
}

func TestProposalIntentContextFailsClosed(t *testing.T) {
    p := validIntent()
    cases := map[string]func(*Context){
        "network": func(c *Context) { c.Network = "other" },
        "chain": func(c *Context) { c.ChainID = "other" },
        "genesis": func(c *Context) { c.GenesisSHA256 = strings.Repeat("c", 64) },
        "future": func(c *Context) { c.CurrentHeight = 99 },
        "expired": func(c *Context) { c.CurrentHeight = 201 },
    }
    for name, mutate := range cases {
        t.Run(name, func(t *testing.T) {
            ctx := validContext()
            mutate(&ctx)
            if err := ValidateForContext(p, ctx); err == nil { t.Fatal("unexpected acceptance") }
        })
    }
    for _, height := range []uint64{100, 200} {
        c := validContext()
        c.CurrentHeight = height
        if err := ValidateForContext(p, c); err != nil { t.Fatalf("boundary height %d: %v", height, err) }
    }
}

func TestProposalIntentMalformedFailsClosed(t *testing.T) {
    changes := map[string]func(*ProposalIntent){
        "version": func(p *ProposalIntent) { p.Version = "V2" },
        "invalid network": func(p *ProposalIntent) { p.Network = "prism network" },
        "invalid chain": func(p *ProposalIntent) { p.ChainID = "" },
        "uppercase genesis": func(p *ProposalIntent) { p.GenesisSHA256 = strings.Repeat("A", 64) },
        "invalid proposer": func(p *ProposalIntent) { p.Proposer = "a/b" },
        "zero nonce": func(p *ProposalIntent) { p.Nonce = 0 },
        "zero created": func(p *ProposalIntent) { p.CreatedAtHeight = 0 },
        "reversed window": func(p *ProposalIntent) { p.ExpiresAtHeight = 100 },
        "unsupported topic": func(p *ProposalIntent) { p.Topic = "EXECUTE" },
        "short title": func(p *ProposalIntent) { p.Title = "short" },
        "whitespace title": func(p *ProposalIntent) { p.Title += " " },
        "control rationale": func(p *ProposalIntent) { p.Rationale += "\n" },
        "invalid payload": func(p *ProposalIntent) { p.PayloadSHA256 = "bad" },
    }
    for name, mutate := range changes {
        t.Run(name, func(t *testing.T) {
            p := validIntent()
            mutate(&p)
            if err := ValidateShape(p); err == nil { t.Fatal("invalid proposal accepted") }
            if _, err := Commitment(p); err == nil { t.Fatal("invalid proposal committed") }
        })
    }
}

func TestProposalIntentNonceCollisionKey(t *testing.T) {
    p := validIntent()
    a, err := ReplayKey(p)
    if err != nil { t.Fatal(err) }
    p.PayloadSHA256 = strings.Repeat("c",64)
    b, err := ReplayKey(p)
    if err != nil { t.Fatal(err) }
    if a != b { t.Fatal("same proposer/nonce must have same replay key regardless of payload") }
    p.Nonce++
    c, err := ReplayKey(p)
    if err != nil { t.Fatal(err) }
    if a == c { t.Fatal("different nonce must have different replay key") }
}
