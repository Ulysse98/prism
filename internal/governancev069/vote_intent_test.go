package governancev069

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
)

func testVote(t *testing.T, seedByte byte) (AdvisoryVoteIntent, ed25519.PrivateKey) {
	t.Helper()
	p := validIntent()
	proposalDigest, err := Commitment(p)
	if err != nil {
		t.Fatal(err)
	}
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seedByte}, ed25519.SeedSize))
	return AdvisoryVoteIntent{
		Version:           AdvisoryVoteProtocolVersion,
		Network:           p.Network,
		ChainID:           p.ChainID,
		GenesisSHA256:     p.GenesisSHA256,
		ProposalSHA256:    hex.EncodeToString(proposalDigest[:]),
		VoterPublicKeyHex: hex.EncodeToString(key.Public().(ed25519.PublicKey)),
		Nonce:             4,
		CreatedAtHeight:   105,
		ExpiresAtHeight:   195,
		Choice:            VoteYes,
	}, key
}

func testSignedVote(t *testing.T) SignedAdvisoryVote {
	t.Helper()
	vote, key := testVote(t, 7)
	signed, err := SignAdvisoryVote(vote, key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestAdvisoryVoteSignatureAndCanonicalCommitment(t *testing.T) {
	v, key := testVote(t, 7)
	signed, err := SignAdvisoryVote(v, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyAdvisoryVote(signed, validIntent(), validContext()); err != nil {
		t.Fatal(err)
	}
	a, err := AdvisoryVoteCommitment(v)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AdvisoryVoteCommitment(v)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a == [32]byte{} {
		t.Fatal("nondeterministic or zero vote digest")
	}
	// Pinned vector prevents accidental cross-language encoding changes.
	const expected = "4848370b0acfeddfef1afbc61b24dbb7135d2d34419e94572af43ac88207c428"
	if got := hex.EncodeToString(a[:]); got != expected {
		t.Fatalf("advisory vote digest vector got %s want %s", got, expected)
	}
	k, err := VoteReplayKey(v)
	if err != nil {
		t.Fatal(err)
	}
	if k == a {
		t.Fatal("vote digest and replay key must be domain separated")
	}
}

func TestAdvisoryVoteAllFieldsBound(t *testing.T) {
	signed := testSignedVote(t)
	mutations := map[string]func(*AdvisoryVoteIntent){
		"version":  func(v *AdvisoryVoteIntent) { v.Version = "WRONG" },
		"network":  func(v *AdvisoryVoteIntent) { v.Network += "-other" },
		"chain":    func(v *AdvisoryVoteIntent) { v.ChainID += "-other" },
		"genesis":  func(v *AdvisoryVoteIntent) { v.GenesisSHA256 = strings.Repeat("b", 64) },
		"proposal": func(v *AdvisoryVoteIntent) { v.ProposalSHA256 = strings.Repeat("d", 64) },
		"voter":    func(v *AdvisoryVoteIntent) { v.VoterPublicKeyHex = strings.Repeat("a", 64) },
		"nonce":    func(v *AdvisoryVoteIntent) { v.Nonce++ },
		"created":  func(v *AdvisoryVoteIntent) { v.CreatedAtHeight++ },
		"expires":  func(v *AdvisoryVoteIntent) { v.ExpiresAtHeight-- },
		"choice":   func(v *AdvisoryVoteIntent) { v.Choice = VoteNo },
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			tampered := signed
			change(&tampered.Vote)
			if err := VerifyAdvisoryVote(tampered, validIntent(), validContext()); err == nil {
				t.Fatal("tampered vote passed verification")
			}
		})
	}
	t.Run("signature", func(t *testing.T) {
		altered := signed
		altered.SignatureHex = strings.Repeat("f", 128)
		if VerifyAdvisoryVote(altered, validIntent(), validContext()) == nil {
			t.Fatal("forged signature accepted")
		}
	})
}

func TestAdvisoryVoteRejectsMalformedAndWrongContext(t *testing.T) {
	v, key := testVote(t, 7)
	invalid := map[string]func(*AdvisoryVoteIntent){
		"nonce zero":           func(v *AdvisoryVoteIntent) { v.Nonce = 0 },
		"window reversed":      func(v *AdvisoryVoteIntent) { v.ExpiresAtHeight = v.CreatedAtHeight },
		"bad vote choice":      func(v *AdvisoryVoteIntent) { v.Choice = "EXECUTE" },
		"uppercase public key": func(v *AdvisoryVoteIntent) { v.VoterPublicKeyHex = strings.ToUpper(v.VoterPublicKeyHex) },
		"bad digest":           func(v *AdvisoryVoteIntent) { v.ProposalSHA256 = "deadbeef" },
	}
	for name, mutate := range invalid {
		t.Run(name, func(t *testing.T) {
			altered := v
			mutate(&altered)
			if _, err := SignAdvisoryVote(altered, key); err == nil {
				t.Fatal("malformed vote signed")
			}
		})
	}
	signed, err := SignAdvisoryVote(v, key)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{5}, ed25519.SeedSize))
	if _, err := SignAdvisoryVote(v, wrongKey); err == nil {
		t.Fatal("mismatched key signed")
	}
	if _, err := SignAdvisoryVote(v, []byte("bad")); err == nil {
		t.Fatal("short private key accepted")
	}

	for name, change := range map[string]func(*Context){
		"network":   func(c *Context) { c.Network = "wrong" },
		"chain":     func(c *Context) { c.ChainID = "wrong" },
		"genesis":   func(c *Context) { c.GenesisSHA256 = strings.Repeat("d", 64) },
		"too early": func(c *Context) { c.CurrentHeight = 104 },
		"too late":  func(c *Context) { c.CurrentHeight = 196 },
	} {
		t.Run(name, func(t *testing.T) {
			ctx := validContext()
			change(&ctx)
			if err := VerifyAdvisoryVote(signed, validIntent(), ctx); err == nil {
				t.Fatal("wrong vote context accepted")
			}
		})
	}
	p := validIntent()
	p.Nonce++
	if err := VerifyAdvisoryVote(signed, p, validContext()); err == nil {
		t.Fatal("wrong proposal accepted")
	}
	for _, height := range []uint64{105, 195} {
		ctx := validContext()
		ctx.CurrentHeight = height
		if err := VerifyAdvisoryVote(signed, validIntent(), ctx); err != nil {
			t.Fatalf("valid boundary %d rejected: %v", height, err)
		}
	}
	v.CreatedAtHeight = 99
	s, err := SignAdvisoryVote(v, key)
	if err != nil {
		t.Fatal(err)
	}
	if VerifyAdvisoryVote(s, validIntent(), validContext()) == nil {
		t.Fatal("vote before proposal window accepted")
	}
	v.CreatedAtHeight = 105
	v.ExpiresAtHeight = 201
	s, err = SignAdvisoryVote(v, key)
	if err != nil {
		t.Fatal(err)
	}
	if VerifyAdvisoryVote(s, validIntent(), validContext()) == nil {
		t.Fatal("vote after proposal expiry accepted")
	}
}

func TestAdvisoryReplayKeyOneVoterOneProposal(t *testing.T) {
	original, _ := testVote(t, 7)
	initial, err := VoteReplayKey(original)
	if err != nil {
		t.Fatal(err)
	}
	altered := original
	altered.Choice = VoteNo
	altered.Nonce++
	alternate, err := VoteReplayKey(altered)
	if err != nil {
		t.Fatal(err)
	}
	if initial != alternate {
		t.Fatal("same voter proposal can evade replay reservation with nonce/choice")
	}
	altered = original
	altered.ProposalSHA256 = strings.Repeat("e", 64)
	other, err := VoteReplayKey(altered)
	if err != nil {
		t.Fatal(err)
	}
	if initial == other {
		t.Fatal("proposal separation missing")
	}
	altered = original
	altered.ChainID += "-other"
	other, err = VoteReplayKey(altered)
	if err != nil {
		t.Fatal(err)
	}
	if initial == other {
		t.Fatal("chain separation missing")
	}
	altered, _ = testVote(t, 8)
	other, err = VoteReplayKey(altered)
	if err != nil {
		t.Fatal(err)
	}
	if initial == other {
		t.Fatal("voter separation missing")
	}
	// A changed nonce cannot change the identity of the reservation.
}
