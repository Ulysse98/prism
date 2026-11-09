//go:build linux

package governancev069

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func foreignClosureKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("PRISM v0.69 PHASE1B7 WRONG TEST AUTHORITY ONLY"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func TestClosure1B7RequiresExternallyPinnedAuthority(t *testing.T) {
	store, receipt, snap, snapPin, ballotPin, ballots, proposal, ctx := closeFixture(t)
	draft, err := PrepareLocalAdvisoryClosureDraft(store, receipt, snap, snapPin, ballotPin, proposal, ctx, ballots)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Authorization != (AdvisoryClosureAuthorization{}) {
		t.Fatal("unsigned draft contains an authority")
	}
	auth, err := SignAdvisoryClosureAuthorization(draft, 7, closureOperatorKey())
	if err != nil {
		t.Fatal(err)
	}
	seal := func(a AdvisoryClosureAuthorization, pin string) error {
		_, err := SealLocalAdvisoryVoteStore(store, receipt, snap, snapPin, ballotPin, proposal, ctx, ballots, a, pin)
		return err
	}
	checks := []struct {
		name  string
		alter func(AdvisoryClosureAuthorization) AdvisoryClosureAuthorization
		pin   string
	}{
		{"missing signature", func(a AdvisoryClosureAuthorization) AdvisoryClosureAuthorization { a.SignatureHex = ""; return a }, closureOperatorPin()},
		{"changed nonce", func(a AdvisoryClosureAuthorization) AdvisoryClosureAuthorization { a.Nonce++; return a }, closureOperatorPin()},
		{"other proposal digest", func(a AdvisoryClosureAuthorization) AdvisoryClosureAuthorization {
			a.ClosureSHA256 = strings.Repeat("0", 64)
			return a
		}, closureOperatorPin()},
		{"fake signature", func(a AdvisoryClosureAuthorization) AdvisoryClosureAuthorization {
			a.SignatureHex = strings.Repeat("0", 128)
			return a
		}, closureOperatorPin()},
		{"wrong pinned public key", func(a AdvisoryClosureAuthorization) AdvisoryClosureAuthorization { return a }, hex.EncodeToString(foreignClosureKey().Public().(ed25519.PublicKey))},
		{"missing independent pin", func(a AdvisoryClosureAuthorization) AdvisoryClosureAuthorization { return a }, ""},
	}
	for _, test := range checks {
		t.Run(test.name, func(t *testing.T) {
			if err := seal(test.alter(auth), test.pin); err == nil {
				t.Fatal("invalid authority accepted")
			}
			if _, err := os.Lstat(filepath.Join(store.dir, localBallotClosureFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe marker created: %v", err)
			}
		})
	}
	badKeyAuth, err := SignAdvisoryClosureAuthorization(draft, 7, foreignClosureKey())
	if err != nil {
		t.Fatal(err)
	}
	if err := seal(badKeyAuth, closureOperatorPin()); err == nil {
		t.Fatal("signer substitution accepted")
	}
	if err := seal(auth, closureOperatorPin()); err != nil {
		t.Fatal(err)
	}
	reopen, err := OpenAdvisoryVoteReplayStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySealedLocalAdvisoryVoteStore(reopen, receipt, snap, snapPin, ballotPin, proposal, ctx, ballots, closureOperatorPin()); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySealedLocalAdvisoryVoteStore(reopen, receipt, snap, snapPin, ballotPin, proposal, ctx, ballots, hex.EncodeToString(foreignClosureKey().Public().(ed25519.PublicKey))); err == nil {
		t.Fatal("different authority trusted on recovery")
	}
	if _, err := reopen.ReserveVerifiedAdvisoryVote(ballots[0], proposal, ctx); !errors.Is(err, ErrAdvisoryBallotClosed) {
		t.Fatalf("restarted writer bypassed seal: %v", err)
	}
}

func TestClosure1B7RejectsStaleOfflineAuthorization(t *testing.T) {
	snap, keys := snapshotFixture(t)
	proposal, ctx := validIntent(), validContext()
	snapPin := snapshotPin(t, snap)
	empty, _ := AdvisoryBallotSetCommitment(nil)
	ballotPin := hex.EncodeToString(empty[:])
	receipt, err := BuildAdvisoryTallyReceipt(snap, snapPin, ballotPin, proposal, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenAdvisoryVoteReplayStore(filepath.Join(t.TempDir(), "stale"))
	if err != nil {
		t.Fatal(err)
	}
	draft, err := PrepareLocalAdvisoryClosureDraft(store, receipt, snap, snapPin, ballotPin, proposal, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := SignAdvisoryClosureAuthorization(draft, 9, closureOperatorKey())
	if err != nil {
		t.Fatal(err)
	}
	vote := signedFixture(t, keys[0], VoteYes)
	if _, err := store.ReserveVerifiedAdvisoryVote(vote, proposal, ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := SealLocalAdvisoryVoteStore(store, receipt, snap, snapPin, ballotPin, proposal, ctx, nil, auth, closureOperatorPin()); err == nil {
		t.Fatal("stale draft accepted after admission")
	}
	if _, err := os.Lstat(filepath.Join(store.dir, localBallotClosureFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("marker created after stale draft")
	}
}

func TestClosure1B7RecoveryInspection(t *testing.T) {
	store, receipt, snap, snapPin, ballotPin, ballots, proposal, ctx := closeFixture(t)
	absent, err := InspectLocalAdvisoryClosureRecovery(store, closureOperatorPin())
	if err != nil || absent.Status != AdvisoryClosureRecoveryAbsent {
		t.Fatalf("unexpected preclose status %+v %v", absent, err)
	}
	sealed, err := closeCall(store, receipt, snap, snapPin, ballotPin, ballots, proposal, ctx)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAdvisoryVoteReplayStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := InspectLocalAdvisoryClosureRecovery(reopened, closureOperatorPin())
	if err != nil || verified.Status != AdvisoryClosureRecoveryVerified || verified.ClosureSHA256 != sealed.ClosureSHA256 {
		t.Fatalf("unexpected recovery %+v %v", verified, err)
	}
	wrong, err := InspectLocalAdvisoryClosureRecovery(reopened, hex.EncodeToString(foreignClosureKey().Public().(ed25519.PublicKey)))
	if err == nil || wrong.Status != AdvisoryClosureRecoveryUnverifiable {
		t.Fatal("wrong pin not rejected")
	}
	path := filepath.Join(store.dir, localBallotClosureFilename)
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	corrupt, err := InspectLocalAdvisoryClosureRecovery(reopened, closureOperatorPin())
	if err == nil || corrupt.Status != AdvisoryClosureRecoveryUnverifiable {
		t.Fatal("corrupted marker not rejected")
	}
	if _, err := reopened.ReserveVerifiedAdvisoryVote(ballots[0], proposal, ctx); !errors.Is(err, ErrAdvisoryBallotClosed) {
		t.Fatal("corrupt marker reopened vote admission")
	}
}
