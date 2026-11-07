package governancev069

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func receiptInputs(t *testing.T) (EligibilitySnapshot, string, []SignedAdvisoryVote, string) {
	t.Helper()
	s, keys := snapshotFixture(t)
	votes := []SignedAdvisoryVote{
		signedFixture(t, keys[0], VoteYes),
		signedFixture(t, keys[1], VoteNo),
		signedFixture(t, keys[2], VoteAbstain),
	}
	set, err := AdvisoryBallotSetCommitment(votes)
	if err != nil {
		t.Fatal(err)
	}
	return s, snapshotPin(t, s), votes, hex.EncodeToString(set[:])
}

func TestAdvisoryReceiptDeterministicAcrossPermutation(t *testing.T) {
	s, pin, votes, ballotPin := receiptInputs(t)
	a, err := BuildAdvisoryTallyReceipt(s, pin, ballotPin, validIntent(), validContext(), votes)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []SignedAdvisoryVote{votes[2], votes[1], votes[0]}
	b, err := BuildAdvisoryTallyReceipt(s, pin, ballotPin, validIntent(), validContext(), reversed)
	if err != nil || a != b {
		t.Fatalf("order-dependent receipt: %+v %+v err %v", a, b, err)
	}
	if a.Tally.YesWeight != 2 || a.Tally.NoWeight != 3 || a.Tally.AbstainWeight != 4 || !a.Tally.QuorumMet || a.Tally.AdvisoryPassed {
		t.Fatalf("incorrect tally: %+v", a.Tally)
	}
	if err := VerifyAdvisoryTallyReceipt(a, s, pin, ballotPin, validIntent(), validContext(), reversed); err != nil {
		t.Fatal(err)
	}
	if len(a.ReceiptSHA256) != 64 || a.ReceiptSHA256 == a.BallotSetSHA256 || a.ReceiptSHA256 == pin {
		t.Fatal("unbound receipt")
	}
	const expectedBallotSet = "edb4fc26db869d6c7b1d69461cf3f27b979651648c6151ffc47b7bd7ea4993c4"
	const expectedReceipt = "4e488cc1efc319d31a2ed031275850c3181a69c1bce761dcc60a0bbd5ec2216e"
	if ballotPin != expectedBallotSet || a.ReceiptSHA256 != expectedReceipt {
		t.Fatalf("canonical vectors: ballot=%s receipt=%s", ballotPin, a.ReceiptSHA256)
	}
}

func TestAdvisoryReceiptFailsClosedOnMutations(t *testing.T) {
	s, pin, votes, ballotPin := receiptInputs(t)
	baseline, err := BuildAdvisoryTallyReceipt(s, pin, ballotPin, validIntent(), validContext(), votes)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*AdvisoryTallyReceipt){
		"version":      func(r *AdvisoryTallyReceipt) { r.Version = "BAD" },
		"network":      func(r *AdvisoryTallyReceipt) { r.Network += "-bad" },
		"chain":        func(r *AdvisoryTallyReceipt) { r.ChainID += "-bad" },
		"genesis":      func(r *AdvisoryTallyReceipt) { r.GenesisSHA256 = strings.Repeat("b", 64) },
		"proposal":     func(r *AdvisoryTallyReceipt) { r.ProposalSHA256 = strings.Repeat("a", 64) },
		"snapshot":     func(r *AdvisoryTallyReceipt) { r.SnapshotSHA256 = strings.Repeat("a", 64) },
		"ballots":      func(r *AdvisoryTallyReceipt) { r.BallotSetSHA256 = strings.Repeat("a", 64) },
		"height":       func(r *AdvisoryTallyReceipt) { r.ObservedAtHeight++ },
		"quorum":       func(r *AdvisoryTallyReceipt) { r.QuorumBps++ },
		"approval":     func(r *AdvisoryTallyReceipt) { r.ApprovalBps++ },
		"eligible":     func(r *AdvisoryTallyReceipt) { r.Tally.EligibleWeight++ },
		"present":      func(r *AdvisoryTallyReceipt) { r.Tally.PresentWeight++ },
		"yes":          func(r *AdvisoryTallyReceipt) { r.Tally.YesWeight++ },
		"no":           func(r *AdvisoryTallyReceipt) { r.Tally.NoWeight++ },
		"abstain":      func(r *AdvisoryTallyReceipt) { r.Tally.AbstainWeight++ },
		"participants": func(r *AdvisoryTallyReceipt) { r.Tally.Participants++ },
		"quorum met":   func(r *AdvisoryTallyReceipt) { r.Tally.QuorumMet = !r.Tally.QuorumMet },
		"approved":     func(r *AdvisoryTallyReceipt) { r.Tally.ApprovalMet = !r.Tally.ApprovalMet },
		"passed":       func(r *AdvisoryTallyReceipt) { r.Tally.AdvisoryPassed = !r.Tally.AdvisoryPassed },
		"digest":       func(r *AdvisoryTallyReceipt) { r.ReceiptSHA256 = strings.Repeat("a", 64) },
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			modified := baseline
			change(&modified)
			if err := VerifyAdvisoryTallyReceipt(modified, s, pin, ballotPin, validIntent(), validContext(), votes); err == nil {
				t.Fatal("altered receipt accepted")
			}
		})
	}
}

func TestAdvisoryReceiptRejectsUntrustedSources(t *testing.T) {
	s, pin, votes, ballotPin := receiptInputs(t)
	p, ctx := validIntent(), validContext()
	badCase := func(name string, snap EligibilitySnapshot, snapPin string, ballotHash string, proposal ProposalIntent, c Context, records []SignedAdvisoryVote) {
		t.Run(name, func(t *testing.T) {
			r, err := BuildAdvisoryTallyReceipt(snap, snapPin, ballotHash, proposal, c, records)
			if err == nil || r != (AdvisoryTallyReceipt{}) {
				t.Fatalf("accepted tampering: %+v %v", r, err)
			}
		})
	}
	badCase("missing ballot pin", s, pin, "", p, ctx, votes)
	badCase("uppercase ballot pin", s, pin, strings.ToUpper(ballotPin), p, ctx, votes)
	badCase("wrong ballot pin", s, pin, strings.Repeat("a", 64), p, ctx, votes)
	badCase("wrong snapshot pin", s, strings.Repeat("a", 64), ballotPin, p, ctx, votes)
	modifiedSnap := s
	modifiedSnap.Members = append([]EligibleVoter(nil), s.Members...)
	modifiedSnap.Members[0].Weight++
	badCase("modified roster", modifiedSnap, pin, ballotPin, p, ctx, votes)
	modifiedP := p
	modifiedP.Nonce++
	badCase("wrong proposal", s, pin, ballotPin, modifiedP, ctx, votes)
	modifiedCtx := ctx
	modifiedCtx.ChainID = "wrong"
	badCase("wrong chain", s, pin, ballotPin, p, modifiedCtx, votes)
	modifiedCtx = ctx
	modifiedCtx.CurrentHeight = 201
	badCase("expired proposal", s, pin, ballotPin, p, modifiedCtx, votes)
	modifiedCtx = ctx
	modifiedCtx.CurrentHeight = 101
	badCase("future votes", s, pin, ballotPin, p, modifiedCtx, votes)
	badSig := append([]SignedAdvisoryVote(nil), votes...)
	badSig[0].SignatureHex = strings.Repeat("0", 128)
	badCase("bad signature", s, pin, ballotPin, p, ctx, badSig)
	badCase("missing ballot", s, pin, ballotPin, p, ctx, votes[:2])
	badCase("duplicate voter", s, pin, ballotPin, p, ctx, append(append([]SignedAdvisoryVote(nil), votes...), votes[0]))
	outsider := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{99}, ed25519.SeedSize))
	badCase("ineligible voter", s, pin, ballotPin, p, ctx, append([]SignedAdvisoryVote{signedFixture(t, outsider, VoteYes)}, votes[1:]...))
	change := votes[0].Vote
	change.Nonce++
	changedVote, _ := SignAdvisoryVote(change, func() ed25519.PrivateKey { _, keys := snapshotFixture(t); return keys[0] }())
	badCase("changed nonce", s, pin, ballotPin, p, ctx, append([]SignedAdvisoryVote{changedVote}, votes[1:]...))
}

func TestBallotSetNoncanonicalAndDuplicateRejected(t *testing.T) {
	_, _, votes, _ := receiptInputs(t)
	bad := append([]SignedAdvisoryVote(nil), votes...)
	bad[0].SignatureHex = strings.ToUpper(bad[0].SignatureHex)
	if _, err := AdvisoryBallotSetCommitment(bad); err == nil {
		t.Fatal("uppercase signature accepted")
	}
	bad = append([]SignedAdvisoryVote(nil), votes...)
	bad[0].SignatureHex = "00"
	if _, err := AdvisoryBallotSetCommitment(bad); err == nil {
		t.Fatal("truncated signature accepted")
	}
	bad = append([]SignedAdvisoryVote(nil), votes...)
	bad[0].Vote.Choice = "EXECUTE"
	if _, err := AdvisoryBallotSetCommitment(bad); err == nil {
		t.Fatal("unknown choice accepted")
	}
	if _, err := AdvisoryBallotSetCommitment(append(votes, votes[0])); err == nil {
		t.Fatal("duplicate accepted")
	}
	oversized := make([]SignedAdvisoryVote, MaxAdvisoryVoters+1)
	if _, err := AdvisoryBallotSetCommitment(oversized); err == nil {
		t.Fatal("oversized accepted")
	}
}

func TestBallotSetBindsSignatureBytesAndAllPermutations(t *testing.T) {
	_, _, votes, hash := receiptInputs(t)
	permutations := [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, perm := range permutations {
		input := []SignedAdvisoryVote{votes[perm[0]], votes[perm[1]], votes[perm[2]]}
		digest, err := AdvisoryBallotSetCommitment(input)
		if err != nil || hex.EncodeToString(digest[:]) != hash {
			t.Fatalf("order mismatch %v: %x %v", perm, digest, err)
		}
	}
	altered := append([]SignedAdvisoryVote(nil), votes...)
	altered[0].SignatureHex = strings.Repeat("0", ed25519.SignatureSize*2)
	digest, err := AdvisoryBallotSetCommitment(altered)
	if err != nil || hex.EncodeToString(digest[:]) == hash {
		t.Fatal("signature bytes not bound")
	}
}

func TestEmptyBallotSetAndLargeWeights(t *testing.T) {
	s, _, _, _ := receiptInputs(t)
	pin := snapshotPin(t, s)
	set, err := AdvisoryBallotSetCommitment(nil)
	if err != nil {
		t.Fatal(err)
	}
	emptyPin := hex.EncodeToString(set[:])
	r, err := BuildAdvisoryTallyReceipt(s, pin, emptyPin, validIntent(), validContext(), nil)
	if err != nil || r.Tally.AdvisoryPassed || r.Tally.Participants != 0 || r.Tally.QuorumMet {
		t.Fatalf("empty vote receipt: %+v %v", r, err)
	}
	if err := VerifyAdvisoryTallyReceipt(r, s, pin, emptyPin, validIntent(), validContext(), nil); err != nil {
		t.Fatal(err)
	}
	large, keys := snapshotFixture(t)
	large.Members[0].Weight = math.MaxUint64 - 2
	large.Members[1].Weight = 1
	large.Members[2].Weight = 1
	// The roster is sorted by key, not by keys slice order; weights remain bounded.
	votes := []SignedAdvisoryVote{signedFixture(t, keys[0], VoteYes), signedFixture(t, keys[1], VoteYes), signedFixture(t, keys[2], VoteYes)}
	set, err = AdvisoryBallotSetCommitment(votes)
	if err != nil {
		t.Fatal(err)
	}
	r, err = BuildAdvisoryTallyReceipt(large, snapshotPin(t, large), hex.EncodeToString(set[:]), validIntent(), validContext(), votes)
	if err != nil || !r.Tally.AdvisoryPassed || r.Tally.PresentWeight != math.MaxUint64 {
		t.Fatalf("large roster receipt: %+v %v", r, err)
	}
}
