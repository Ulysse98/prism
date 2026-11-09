package governancev069

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"testing"
)

func snapshotFixture(t *testing.T) (EligibilitySnapshot, []ed25519.PrivateKey) {
	t.Helper()
	p := validIntent()
	hash, err := Commitment(p)
	if err != nil {
		t.Fatal(err)
	}
	s := EligibilitySnapshot{Version: EligibilitySnapshotVersion, Network: p.Network, ChainID: p.ChainID,
		GenesisSHA256: p.GenesisSHA256, ProposalSHA256: hex.EncodeToString(hash[:]),
		SnapshotHeight: p.CreatedAtHeight, QuorumBps: 6000, ApprovalBps: 6000}
	keys := make([]ed25519.PrivateKey, 3)
	for i := range keys {
		keys[i] = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(10 + i)}, ed25519.SeedSize))
		s.Members = append(s.Members, EligibleVoter{PublicKeyHex: hex.EncodeToString(keys[i].Public().(ed25519.PublicKey)), Weight: uint64(2 + i)})
	}
	sort.Slice(s.Members, func(i, j int) bool { return s.Members[i].PublicKeyHex < s.Members[j].PublicKeyHex })
	return s, keys
}

func snapshotPin(t *testing.T, s EligibilitySnapshot) string {
	t.Helper()
	h, err := EligibilitySnapshotCommitment(s)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h[:])
}

func signedFixture(t *testing.T, k ed25519.PrivateKey, choice VoteChoice) SignedAdvisoryVote {
	t.Helper()
	p := validIntent()
	h, err := Commitment(p)
	if err != nil {
		t.Fatal(err)
	}
	v := AdvisoryVoteIntent{Version: AdvisoryVoteProtocolVersion, Network: p.Network, ChainID: p.ChainID,
		GenesisSHA256: p.GenesisSHA256, ProposalSHA256: hex.EncodeToString(h[:]),
		VoterPublicKeyHex: hex.EncodeToString(k.Public().(ed25519.PublicKey)), Nonce: 42,
		CreatedAtHeight: 105, ExpiresAtHeight: 195, Choice: choice}
	signed, err := SignAdvisoryVote(v, k)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestEligibilityTallyQuorumAndApproval(t *testing.T) {
	s, keys := snapshotFixture(t)
	pin := snapshotPin(t, s)
	all := []SignedAdvisoryVote{signedFixture(t, keys[0], VoteYes), signedFixture(t, keys[1], VoteNo), signedFixture(t, keys[2], VoteAbstain)}
	got, err := TallyAdvisoryVotes(s, pin, validIntent(), validContext(), all)
	if err != nil {
		t.Fatal(err)
	}
	// 2 yes, 3 no, 4 abstain => quorum 9/9; approval 2/5 < 60%.
	if got.EligibleWeight != 9 || got.PresentWeight != 9 || got.YesWeight != 2 || got.NoWeight != 3 || got.AbstainWeight != 4 ||
		got.Participants != 3 || !got.QuorumMet || got.ApprovalMet || got.AdvisoryPassed {
		t.Fatalf("bad tally %+v", got)
	}
	got, err = TallyAdvisoryVotes(s, pin, validIntent(), validContext(), []SignedAdvisoryVote{all[0], all[2]})
	if err != nil {
		t.Fatal(err)
	}
	if !got.QuorumMet || !got.ApprovalMet || !got.AdvisoryPassed {
		t.Fatalf("2 yes + 4 abstain should pass advisory rule: %+v", got)
	}
	got, err = TallyAdvisoryVotes(s, pin, validIntent(), validContext(), []SignedAdvisoryVote{all[2]})
	if err != nil {
		t.Fatal(err)
	}
	if got.QuorumMet || got.ApprovalMet || got.AdvisoryPassed {
		t.Fatal("abstention alone cannot approve")
	}
	got, err = TallyAdvisoryVotes(s, pin, validIntent(), validContext(), nil)
	if err != nil || got.PresentWeight != 0 || got.QuorumMet || got.ApprovalMet {
		t.Fatalf("empty tally: %+v %v", got, err)
	}
}

func TestEligibilitySnapshotCanonicalAndMutations(t *testing.T) {
	s, _ := snapshotFixture(t)
	base := snapshotPin(t, s)
	const pinnedVector = "657ef81c639fd1e0e2fb3a99470d14ab6e2239779f6d4f4a8f1176854fc4af4d"
	if base != pinnedVector {
		t.Fatalf("eligibility commitment vector changed: got %s", base)
	}
	if base != snapshotPin(t, s) {
		t.Fatal("non-deterministic digest")
	}
	changes := map[string]func(*EligibilitySnapshot){
		"version":  func(x *EligibilitySnapshot) { x.Version = "BAD" },
		"network":  func(x *EligibilitySnapshot) { x.Network += "-alt" },
		"chain":    func(x *EligibilitySnapshot) { x.ChainID += "-alt" },
		"genesis":  func(x *EligibilitySnapshot) { x.GenesisSHA256 = strings.Repeat("b", 64) },
		"proposal": func(x *EligibilitySnapshot) { x.ProposalSHA256 = strings.Repeat("c", 64) },
		"height":   func(x *EligibilitySnapshot) { x.SnapshotHeight++ },
		"quorum":   func(x *EligibilitySnapshot) { x.QuorumBps++ },
		"approval": func(x *EligibilitySnapshot) { x.ApprovalBps++ },
		"member":   func(x *EligibilitySnapshot) { x.Members[0].Weight++ },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			altered := s
			altered.Members = append([]EligibleVoter(nil), s.Members...)
			change(&altered)
			got, err := EligibilitySnapshotCommitment(altered)
			if err == nil && hex.EncodeToString(got[:]) == base {
				t.Fatal("mutation did not change commitment")
			}
		})
	}
}

func TestEligibilitySnapshotRejectsBadShape(t *testing.T) {
	s, _ := snapshotFixture(t)
	bad := map[string]func(*EligibilitySnapshot){
		"nil members":   func(x *EligibilitySnapshot) { x.Members = nil },
		"zero weight":   func(x *EligibilitySnapshot) { x.Members[0].Weight = 0 },
		"duplicate":     func(x *EligibilitySnapshot) { x.Members[1].PublicKeyHex = x.Members[0].PublicKeyHex },
		"unordered":     func(x *EligibilitySnapshot) { x.Members[0], x.Members[1] = x.Members[1], x.Members[0] },
		"uppercase key": func(x *EligibilitySnapshot) { x.Members[0].PublicKeyHex = strings.ToUpper(x.Members[0].PublicKeyHex) },
		"overflow":      func(x *EligibilitySnapshot) { x.Members[0].Weight = math.MaxUint64 },
		"zero height":   func(x *EligibilitySnapshot) { x.SnapshotHeight = 0 },
		"zero quorum":   func(x *EligibilitySnapshot) { x.QuorumBps = 0 },
		"over quorum":   func(x *EligibilitySnapshot) { x.QuorumBps = 10001 },
		"weak approval": func(x *EligibilitySnapshot) { x.ApprovalBps = 5000 },
		"over approval": func(x *EligibilitySnapshot) { x.ApprovalBps = 10001 },
		"bad hash":      func(x *EligibilitySnapshot) { x.GenesisSHA256 = "bad" },
	}
	for name, change := range bad {
		t.Run(name, func(t *testing.T) {
			x := s
			x.Members = append([]EligibleVoter(nil), s.Members...)
			change(&x)
			if err := ValidateEligibilitySnapshot(x); err == nil {
				t.Fatal("accepted invalid eligibility snapshot")
			}
		})
	}
	many := s
	many.Members = make([]EligibleVoter, MaxAdvisoryVoters+1)
	if ValidateEligibilitySnapshot(many) == nil {
		t.Fatal("accepted oversized roster")
	}
}

func TestEligibilityTallyFailsClosedOnUntrustedInputs(t *testing.T) {
	s, keys := snapshotFixture(t)
	pin := snapshotPin(t, s)
	ok := signedFixture(t, keys[0], VoteYes)
	baseP := validIntent()
	baseC := validContext()
	testCase := func(name string, snap EligibilitySnapshot, pinArg string, p ProposalIntent, ctx Context, votes []SignedAdvisoryVote) {
		t.Run(name, func(t *testing.T) {
			_, err := TallyAdvisoryVotes(snap, pinArg, p, ctx, votes)
			if err == nil {
				t.Fatal("untrusted tally accepted")
			}
		})
	}
	testCase("wrong pin", s, strings.Repeat("a", 64), baseP, baseC, []SignedAdvisoryVote{ok})
	testCase("uppercase pin", s, strings.ToUpper(pin), baseP, baseC, nil)
	changed := s
	changed.Network += "-other"
	testCase("wrong network", changed, snapshotPin(t, changed), baseP, baseC, nil)
	changed = s
	changed.ChainID += "-other"
	testCase("wrong chain", changed, snapshotPin(t, changed), baseP, baseC, nil)
	changed = s
	changed.GenesisSHA256 = strings.Repeat("c", 64)
	testCase("wrong genesis", changed, snapshotPin(t, changed), baseP, baseC, nil)
	changed = s
	changed.SnapshotHeight++
	testCase("floating roster height", changed, snapshotPin(t, changed), baseP, baseC, nil)
	changed = s
	changed.ProposalSHA256 = strings.Repeat("b", 64)
	testCase("wrong proposal", changed, snapshotPin(t, changed), baseP, baseC, nil)
	wrongProposal := baseP
	wrongProposal.Nonce++
	testCase("changed proposal", s, pin, wrongProposal, baseC, nil)
	wrongCtx := baseC
	wrongCtx.CurrentHeight = 201
	testCase("expired proposal", s, pin, baseP, wrongCtx, nil)
	altered := ok
	altered.SignatureHex = strings.Repeat("f", 128)
	testCase("bad signature", s, pin, baseP, baseC, []SignedAdvisoryVote{altered})
	outsider := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{90}, ed25519.SeedSize))
	testCase("not eligible", s, pin, baseP, baseC, []SignedAdvisoryVote{signedFixture(t, outsider, VoteYes)})
	duplicate := signedFixture(t, keys[0], VoteNo)
	duplicate.Vote.Nonce++
	duplicate, _ = SignAdvisoryVote(duplicate.Vote, keys[0])
	testCase("duplicate nonce+choice", s, pin, baseP, baseC, []SignedAdvisoryVote{ok, duplicate})
	// No partial results if a later vote is invalid.
	if got, err := TallyAdvisoryVotes(s, pin, baseP, baseC, []SignedAdvisoryVote{ok, altered}); err == nil || got != (AdvisoryTally{}) {
		t.Fatalf("returned partial tally: %+v %v", got, err)
	}
}

func TestEligibilityExactThresholdAndLargeWeight(t *testing.T) {
	if !atLeastBps(6, 10, 6000) || atLeastBps(5, 10, 6000) || !atLeastBps(math.MaxUint64, math.MaxUint64, 10000) {
		t.Fatal("incorrect exact threshold")
	}
	if atLeastBps(math.MaxUint64-1, math.MaxUint64, 10000) || atLeastBps(0, 0, 5001) {
		t.Fatal("incorrect denominator or rounding")
	}
	s, keys := snapshotFixture(t)
	s.Members[0].Weight = math.MaxUint64 - 2
	s.Members[1].Weight = 1
	s.Members[2].Weight = 1
	pin := snapshotPin(t, s)
	all := []SignedAdvisoryVote{
		signedFixture(t, keys[0], VoteYes), signedFixture(t, keys[1], VoteYes), signedFixture(t, keys[2], VoteYes),
	}
	got, err := TallyAdvisoryVotes(s, pin, validIntent(), validContext(), all)
	if err != nil || !got.AdvisoryPassed || got.PresentWeight != math.MaxUint64 {
		t.Fatalf("large weight tally %+v err %v", got, err)
	}
}
