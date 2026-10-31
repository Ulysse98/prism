//go:build linux

package finality

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func journalTestContextV065() VoteContext {
	return VoteContext{
		ChainID: "prism-lab-v065", GenesisHash: strings.Repeat("a", 64),
		Height: 42, Round: 2, Step: stepPrevote,
		BlockHash: strings.Repeat("b", 64), ValidatorSetHash: strings.Repeat("c", 64),
	}
}

func journalTestKeyV065() ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed([]byte("0123456789abcdef0123456789abcdef"))
}

func privateVoteTestDirV065(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestJournalV065RetryConflictAndRestart(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	ctx := journalTestContextV065()
	key := journalTestKeyV065()
	first, err := SignVoteOnceV065(dir, ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SignVoteOnceV065(dir, ctx, key)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("idempotent retry: %v", err)
	}

	clash := ctx
	clash.BlockHash = strings.Repeat("d", 64)
	if _, err := SignVoteOnceV065(dir, clash, key); err == nil {
		t.Fatal("conflicting vote signed")
	}
	clash = ctx
	clash.ValidatorSetHash = strings.Repeat("e", 64)
	if _, err := SignVoteOnceV065(dir, clash, key); err == nil {
		t.Fatal("changed validator snapshot escaped lock")
	}

	journal, err := readVoteJournalV065(filepath.Join(dir, voteJournalFilenameV065))
	if err != nil || len(journal.Votes) != 1 || journal.Votes[0] != first {
		t.Fatalf("unexpected journal: %v", err)
	}

	// Fresh call opens the persisted journal again, mirroring a restart.
	again, err := SignVoteOnceV065(dir, ctx, key)
	if err != nil || again != first {
		t.Fatalf("persisted retry: %v", err)
	}

	next := ctx
	next.Step = stepPrecommit
	if _, err := SignVoteOnceV065(dir, next, key); err != nil {
		t.Fatal(err)
	}
	next = ctx
	next.Height++
	if _, err := SignVoteOnceV065(dir, next, key); err != nil {
		t.Fatal(err)
	}
}

func TestJournalV065FailClosedOnCorruption(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	ctx := journalTestContextV065()
	key := journalTestKeyV065()
	if _, err := SignVoteOnceV065(dir, ctx, key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, voteJournalFilenameV065)
	if err := os.WriteFile(path, []byte(`{"version":1,"votes":[{"broken":true}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx.Height++
	if _, err := SignVoteOnceV065(dir, ctx, key); err == nil {
		t.Fatal("corrupted journal accepted")
	}
}

func TestJournalV065SameProcessConcurrentConflict(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	key := journalTestKeyV065()
	ctx := journalTestContextV065()
	var wg sync.WaitGroup
	type outcome struct {
		hash string
		err  error
	}
	results := make([]outcome, 16)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			candidate := ctx
			if i%2 == 1 {
				candidate.BlockHash = strings.Repeat("d", 64)
			}
			vote, err := SignVoteOnceV065(dir, candidate, key)
			results[i] = outcome{vote.Context.BlockHash, err}
		}(i)
	}
	wg.Wait()
	ok, rejected := 0, 0
	for _, r := range results {
		if r.err != nil {
			rejected++
		} else {
			ok++
			if r.hash == "" {
				t.Fatal("empty signed vote")
			}
		}
	}
	if ok != 8 || rejected != 8 {
		t.Fatalf("same-process results: %d signed, %d rejected", ok, rejected)
	}
	journal, err := readVoteJournalV065(filepath.Join(dir, voteJournalFilenameV065))
	if err != nil || len(journal.Votes) != 1 {
		t.Fatalf("unexpected persisted votes: %v", err)
	}
}

// Helper runs as a separate OS process to exercise flock, not just goroutines.
func TestJournalV065ChildSigner(t *testing.T) {
	if os.Getenv("PRISM_V065_JOURNAL_CHILD") != "1" {
		return
	}
	ctx := journalTestContextV065()
	if os.Getenv("PRISM_V065_JOURNAL_ALT") == "1" {
		ctx.BlockHash = strings.Repeat("d", 64)
	}
	_, err := SignVoteOnceV065(os.Getenv("PRISM_V065_JOURNAL_DIR"), ctx, journalTestKeyV065())
	if err != nil {
		os.Exit(7)
	}
	os.Exit(0)
}

func TestJournalV065MultiProcessConcurrentConflict(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	var wg sync.WaitGroup
	outcomes := make([]error, 12)
	for i := range outcomes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			alt := "0"
			if i%2 == 1 {
				alt = "1"
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestJournalV065ChildSigner$")
			cmd.Env = append(os.Environ(),
				"PRISM_V065_JOURNAL_CHILD=1", "PRISM_V065_JOURNAL_DIR="+dir,
				"PRISM_V065_JOURNAL_ALT="+alt)
			outcomes[i] = cmd.Run()
		}(i)
	}
	wg.Wait()
	accepted, rejected := 0, 0
	for _, err := range outcomes {
		if err == nil {
			accepted++
			continue
		}
		var exit *exec.ExitError
		if !isExitCodeSevenV065(err, &exit) {
			t.Fatalf("unexpected child error: %v", err)
		}
		rejected++
	}
	if accepted != 6 || rejected != 6 {
		t.Fatalf("multi-process results %d accepted %d rejected", accepted, rejected)
	}
	journal, err := readVoteJournalV065(filepath.Join(dir, voteJournalFilenameV065))
	if err != nil || len(journal.Votes) != 1 {
		t.Fatalf("unexpected persisted journal: %v", err)
	}
}

func isExitCodeSevenV065(err error, target **exec.ExitError) bool {
	if x, ok := err.(*exec.ExitError); ok && x.ExitCode() == 7 {
		*target = x
		return true
	}
	return false
}

func TestJournalV065MalformedStoredSignature(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	ctx := journalTestContextV065()
	key := journalTestKeyV065()
	if _, err := SignVoteOnceV065(dir, ctx, key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, voteJournalFilenameV065)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var j voteJournalV065
	if err := json.Unmarshal(data, &j); err != nil {
		t.Fatal(err)
	}
	j.Votes[0].Signature = strings.Repeat("0", 128)
	altered, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, altered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SignVoteOnceV065(dir, ctx, key); err == nil {
		t.Fatal("tampered journal accepted")
	}
}

func TestJournalV065CrossRoundLockNotImplemented(t *testing.T) {
	dir := privateVoteTestDirV065(t)
	ctx := journalTestContextV065()
	key := journalTestKeyV065()
	if _, err := SignVoteOnceV065(dir, ctx, key); err != nil {
		t.Fatal(err)
	}
	other := ctx
	other.Round++
	other.BlockHash = strings.Repeat("d", 64)
	if _, err := SignVoteOnceV065(dir, other, key); err != nil {
		t.Fatal("cross-round signatures remain unconstrained in this lab:", err)
	}
	t.Log("LIMITATION: a vote on a different round can still target a different block")
}
