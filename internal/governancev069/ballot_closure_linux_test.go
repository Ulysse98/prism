//go:build linux

package governancev069

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func closeFixture(t *testing.T) (*AdvisoryVoteReplayStore, AdvisoryTallyReceipt, EligibilitySnapshot, string, string, []SignedAdvisoryVote, ProposalIntent, Context) {
	t.Helper()
	return auditFixture(t)
}

func closeCall(store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, pin, ballotPin string, votes []SignedAdvisoryVote, p ProposalIntent, ctx Context) (AdvisoryBallotClosure, error) {
	return SealLocalAdvisoryVoteStore(store, receipt, snapshot, pin, ballotPin, p, ctx, votes)
}
func verifyClose(store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, pin, ballotPin string, votes []SignedAdvisoryVote, p ProposalIntent, ctx Context) (AdvisoryBallotClosure, error) {
	return VerifySealedLocalAdvisoryVoteStore(store, receipt, snapshot, pin, ballotPin, p, ctx, votes)
}

func TestLocalBallotClosureDurableAndDeterministic(t *testing.T) {
	store, receipt, snapshot, pin, ballotPin, votes, p, ctx := closeFixture(t)
	c, err := closeCall(store, receipt, snapshot, pin, ballotPin, votes, p, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != AdvisoryBallotClosureVersion || c.TallyReceiptSHA256 != receipt.ReceiptSHA256 || len(c.ClosureSHA256) != 64 {
		t.Fatalf("unexpected closure: %+v", c)
	}
	// Freeze the canonical vector to catch unintended format changes.
	const expected = "290ebedf8f4cbf081a9674d8255316dfbb8736cbfef99ae9abea6397c6b70f02"
	if c.ClosureSHA256 != expected {
		t.Fatalf("canonical closure vector changed: got %s expected %s", c.ClosureSHA256, expected)
	}
	reopened, err := OpenAdvisoryVoteReplayStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	same, err := verifyClose(reopened, receipt, snapshot, pin, ballotPin, []SignedAdvisoryVote{votes[2], votes[0], votes[1]}, p, ctx)
	if err != nil || c != same {
		t.Fatalf("reopened closure differs: %+v %v", same, err)
	}
	if _, err := closeCall(reopened, receipt, snapshot, pin, ballotPin, votes, p, ctx); !errors.Is(err, ErrAdvisoryBallotClosed) {
		t.Fatalf("double closure accepted: %v", err)
	}
	if _, err := reopened.ReserveVerifiedAdvisoryVote(votes[0], p, ctx); !errors.Is(err, ErrAdvisoryBallotClosed) {
		t.Fatalf("post-closure vote accepted: %v", err)
	}
	// Even a different, correctly signed voter must be rejected.
	_, keys := snapshotFixture(t)
	other := signedFixture(t, keys[0], VoteNo)
	other.Vote.Nonce++
	other, _ = SignAdvisoryVote(other.Vote, keys[0])
	if _, err := reopened.ReserveVerifiedAdvisoryVote(other, p, ctx); !errors.Is(err, ErrAdvisoryBallotClosed) {
		t.Fatalf("new ballot accepted: %v", err)
	}
	if _, err := auditCall(reopened, receipt, snapshot, pin, ballotPin, votes, p, ctx); err != nil {
		t.Fatalf("post-closure audit rejected: %v", err)
	}
}

func TestLocalClosureRejectsWrongPinsAndUnexpectedRecords(t *testing.T) {
	cases := []struct {
		name  string
		alter func(*AdvisoryTallyReceipt, *string, *string, *[]SignedAdvisoryVote)
	}{
		{"snapshot pin", func(_ *AdvisoryTallyReceipt, pin, _ *string, _ *[]SignedAdvisoryVote) { *pin = strings.Repeat("a", 64) }},
		{"ballot pin", func(_ *AdvisoryTallyReceipt, _, pin *string, _ *[]SignedAdvisoryVote) { *pin = strings.Repeat("a", 64) }},
		{"tally digest", func(r *AdvisoryTallyReceipt, _, _ *string, _ *[]SignedAdvisoryVote) {
			r.ReceiptSHA256 = strings.Repeat("b", 64)
		}},
		{"missing ballot", func(_ *AdvisoryTallyReceipt, _, _ *string, ballots *[]SignedAdvisoryVote) { *ballots = (*ballots)[:2] }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			store, receipt, snap, pin, ballotPin, ballots, p, ctx := closeFixture(t)
			tt.alter(&receipt, &pin, &ballotPin, &ballots)
			if _, err := closeCall(store, receipt, snap, pin, ballotPin, ballots, p, ctx); err == nil {
				t.Fatal("untrusted input allowed seal")
			}
			if _, err := os.Lstat(filepath.Join(store.dir, localBallotClosureFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected closure marker: %v", err)
			}
		})
	}
	store, receipt, snap, pin, ballotPin, ballots, p, ctx := closeFixture(t)
	if err := os.WriteFile(filepath.Join(store.dir, "unexpected.vote"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := closeCall(store, receipt, snap, pin, ballotPin, ballots, p, ctx); err == nil {
		t.Fatal("unexpected vote allowed seal")
	}
}

func TestLocalClosureFailClosedOnCorruptMarker(t *testing.T) {
	for _, mode := range []string{"partial", "tampered", "permission", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			store, receipt, snap, pin, ballotPin, votes, p, ctx := closeFixture(t)
			path := filepath.Join(store.dir, localBallotClosureFilename)
			if mode != "partial" {
				if _, err := closeCall(store, receipt, snap, pin, ballotPin, votes, p, ctx); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "partial":
				if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
					t.Fatal(err)
				}
			case "tampered":
				if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "permission":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/etc/passwd", path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.ReserveVerifiedAdvisoryVote(votes[0], p, ctx); err == nil {
				t.Fatal("corrupt/ambiguous closure reopened votes")
			}
			if _, err := verifyClose(store, receipt, snap, pin, ballotPin, votes, p, ctx); err == nil {
				t.Fatal("corrupt closure verified")
			}
		})
	}
}

func TestLocalClosureRaceWithVote(t *testing.T) {
	// Race a valid reservation against closure of the pinned EMPTY ballot set.
	// Exactly one can win. If the vote wins the audit must reject closure;
	// if closure wins the vote must be rejected by the durable marker.
	for trial := 0; trial < 12; trial++ {
		snapshot, keys := snapshotFixture(t)
		p, ctx := validIntent(), validContext()
		pin := snapshotPin(t, snapshot)
		digest, err := AdvisoryBallotSetCommitment(nil)
		if err != nil {
			t.Fatal(err)
		}
		ballotPin := hex.EncodeToString(digest[:])
		receipt, err := BuildAdvisoryTallyReceipt(snapshot, pin, ballotPin, p, ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		store, err := OpenAdvisoryVoteReplayStore(filepath.Join(t.TempDir(), "votes"))
		if err != nil {
			t.Fatal(err)
		}
		signed := signedFixture(t, keys[0], VoteYes)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var sealErr, reserveErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, sealErr = closeCall(store, receipt, snapshot, pin, ballotPin, nil, p, ctx)
		}()
		go func() { defer wg.Done(); <-start; _, reserveErr = store.ReserveVerifiedAdvisoryVote(signed, p, ctx) }()
		close(start)
		wg.Wait()
		switch {
		case sealErr == nil && errors.Is(reserveErr, ErrAdvisoryBallotClosed):
			if _, err := verifyClose(store, receipt, snapshot, pin, ballotPin, nil, p, ctx); err != nil {
				t.Fatal(err)
			}
		case reserveErr == nil && sealErr != nil:
			if _, err := os.Lstat(filepath.Join(store.dir, localBallotClosureFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("marker created after inconsistent audit: %v", err)
			}
		default:
			t.Fatalf("unreconciled race: seal %v reserve %v", sealErr, reserveErr)
		}
	}
}

// Re-exec test binary to confirm Linux flock coordination across processes.
func TestLocalClosureChildVote(t *testing.T) {
	if os.Getenv("PRISM_V069_CLOSURE_CHILD") != "1" {
		return
	}
	fixture := os.Getenv("PRISM_V069_CLOSURE_FIXTURE")
	bytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var f processFixture
	if err := json.Unmarshal(bytes, &f); err != nil {
		t.Fatal(err)
	}
	store, err := OpenAdvisoryVoteReplayStore(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ReserveVerifiedAdvisoryVote(f.Signed, f.Proposal, f.Context)
	if errors.Is(err, ErrAdvisoryBallotClosed) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Fatal("child admitted vote after seal")
}

func TestLocalClosureMultiprocessAfterSeal(t *testing.T) {
	store, receipt, snap, pin, ballotPin, votes, p, ctx := closeFixture(t)
	if _, err := closeCall(store, receipt, snap, pin, ballotPin, votes, p, ctx); err != nil {
		t.Fatal(err)
	}
	fixture := processFixture{Signed: votes[0], Proposal: p, Context: ctx, Dir: store.dir}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	const n = 6
	var wg sync.WaitGroup
	failures := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestLocalClosureChildVote$", "-test.v")
			cmd.Env = append(os.Environ(), "PRISM_V069_CLOSURE_CHILD=1", "PRISM_V069_CLOSURE_FIXTURE="+path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				failures <- errors.New(string(out))
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
}
