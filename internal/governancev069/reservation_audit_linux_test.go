//go:build linux

package governancev069

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditFixture(t *testing.T) (*AdvisoryVoteReplayStore, AdvisoryTallyReceipt, EligibilitySnapshot, string, string, []SignedAdvisoryVote, ProposalIntent, Context) {
	t.Helper()
	snapshot, snapPin, votes, ballotPin := receiptInputs(t)
	proposal, ctx := validIntent(), validContext()
	r, err := BuildAdvisoryTallyReceipt(snapshot, snapPin, ballotPin, proposal, ctx, votes)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenAdvisoryVoteReplayStore(filepath.Join(t.TempDir(), "private"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range votes {
		if _, err := store.ReserveVerifiedAdvisoryVote(v, proposal, ctx); err != nil {
			t.Fatal(err)
		}
	}
	return store, r, snapshot, snapPin, ballotPin, votes, proposal, ctx
}

func auditCall(store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, snapPin, ballotPin string, votes []SignedAdvisoryVote, p ProposalIntent, ctx Context) (AdvisoryReservationAudit, error) {
	return AuditLocalAdvisoryReservations(store, receipt, snapshot, snapPin, ballotPin, p, ctx, votes)
}
func TestLocalReservationAuditHappyPathAndDeterminism(t *testing.T) {
	store, receipt, snapshot, pin, ballotPin, votes, p, ctx := auditFixture(t)
	audit, err := auditCall(store, receipt, snapshot, pin, ballotPin, votes, p, ctx)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []SignedAdvisoryVote{votes[2], votes[1], votes[0]}
	repeated, err := auditCall(store, receipt, snapshot, pin, ballotPin, reversed, p, ctx)
	if err != nil || audit != repeated {
		t.Fatalf("audit changed with ballot order: %v; %v", err, audit)
	}
	if audit.MatchedReservations != 3 || audit.BallotSetSHA256 != ballotPin || audit.TallyReceiptSHA256 != receipt.ReceiptSHA256 || len(audit.AuditSHA256) != 64 {
		t.Fatal(audit)
	}
	const expectedAudit = "5b50a2013e1f88178268b2764efd2d454b51100a32c81f9d52268c279f8e2505"
	if audit.AuditSHA256 != expectedAudit {
		t.Fatalf("canonical audit vector changed: got %s want %s", audit.AuditSHA256, expectedAudit)
	}
	if err := VerifyLocalAdvisoryReservationAudit(audit, store, receipt, snapshot, pin, ballotPin, p, ctx, votes); err != nil {
		t.Fatal(err)
	}
	changed := audit
	changed.MatchedReservations++
	if err := VerifyLocalAdvisoryReservationAudit(changed, store, receipt, snapshot, pin, ballotPin, p, ctx, votes); err == nil {
		t.Fatal("modified audit accepted")
	}
}
func TestLocalReservationAuditRejectsOmissionAndExtra(t *testing.T) {
	store, receipt, snapshot, pin, ballotPin, votes, p, ctx := auditFixture(t)
	if _, err := auditCall(store, receipt, snapshot, pin, ballotPin, votes[:2], p, ctx); err == nil {
		t.Fatal("omitted vote accepted")
	}
	if err := os.WriteFile(filepath.Join(store.dir, "unexpected.vote"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := auditCall(store, receipt, snapshot, pin, ballotPin, votes, p, ctx); err == nil {
		t.Fatal("extra file accepted")
	}
}
func TestLocalReservationAuditRejectsTamperAndPermissions(t *testing.T) {
	scenarios := []struct {
		name   string
		mutate func(t *testing.T, store *AdvisoryVoteReplayStore, ballots []SignedAdvisoryVote)
	}{
		{"record content", func(t *testing.T, store *AdvisoryVoteReplayStore, _ []SignedAdvisoryVote) {
			names, _ := os.ReadDir(store.dir)
			path := filepath.Join(store.dir, names[0].Name())
			if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"record permissions", func(t *testing.T, store *AdvisoryVoteReplayStore, _ []SignedAdvisoryVote) {
			names, _ := os.ReadDir(store.dir)
			if err := os.Chmod(filepath.Join(store.dir, names[0].Name()), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"record symlink", func(t *testing.T, store *AdvisoryVoteReplayStore, _ []SignedAdvisoryVote) {
			names, _ := os.ReadDir(store.dir)
			path := filepath.Join(store.dir, names[0].Name())
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("/etc/passwd", path); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory permissions", func(t *testing.T, store *AdvisoryVoteReplayStore, _ []SignedAdvisoryVote) {
			if err := os.Chmod(store.dir, 0755); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			store, receipt, snapshot, pin, ballotPin, votes, p, ctx := auditFixture(t)
			tt.mutate(t, store, votes)
			if _, err := auditCall(store, receipt, snapshot, pin, ballotPin, votes, p, ctx); err == nil {
				t.Fatal("tampered reservation accepted")
			}
		})
	}
}
func TestLocalReservationAuditRejectsWrongPinsAndReceipt(t *testing.T) {
	store, receipt, snapshot, pin, ballotPin, votes, p, ctx := auditFixture(t)
	if _, err := auditCall(store, receipt, snapshot, strings.Repeat("0", 64), ballotPin, votes, p, ctx); err == nil {
		t.Fatal("wrong roster pin accepted")
	}
	if _, err := auditCall(store, receipt, snapshot, pin, strings.Repeat("0", 64), votes, p, ctx); err == nil {
		t.Fatal("wrong ballots pin accepted")
	}
	changed := receipt
	changed.ReceiptSHA256 = strings.Repeat("a", 64)
	if _, err := auditCall(store, changed, snapshot, pin, ballotPin, votes, p, ctx); err == nil {
		t.Fatal("modified receipt accepted")
	}
	changedCtx := ctx
	changedCtx.GenesisSHA256 = strings.Repeat("1", 64)
	if _, err := auditCall(store, receipt, snapshot, pin, ballotPin, votes, p, changedCtx); err == nil {
		t.Fatal("wrong network context accepted")
	}
	if _, err := auditCall(store, receipt, snapshot, pin, ballotPin, append(votes, votes[0]), p, ctx); err == nil {
		t.Fatal("duplicate accepted")
	}
	bad := append([]SignedAdvisoryVote(nil), votes...)
	bad[0].SignatureHex = strings.Repeat("0", 128)
	if _, err := auditCall(store, receipt, snapshot, pin, ballotPin, bad, p, ctx); err == nil {
		t.Fatal("signature tamper accepted")
	}
}
func TestLocalReservationAuditEmptyAndIncomplete(t *testing.T) {
	s, _ := snapshotFixture(t)
	p, ctx := validIntent(), validContext()
	pin := snapshotPin(t, s)
	set, _ := AdvisoryBallotSetCommitment(nil)
	ballotPin := hex.EncodeToString(set[:])
	r, err := BuildAdvisoryTallyReceipt(s, pin, ballotPin, p, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenAdvisoryVoteReplayStore(filepath.Join(t.TempDir(), "empty"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := auditCall(store, r, s, pin, ballotPin, nil, p, ctx)
	if err != nil || got.MatchedReservations != 0 {
		t.Fatalf("empty audit: %+v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(store.dir, strings.Repeat("0", 64)+".vote"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := auditCall(store, r, s, pin, ballotPin, nil, p, ctx); err == nil {
		t.Fatal("crash-incomplete record accepted")
	}
}
