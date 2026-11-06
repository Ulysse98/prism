//go:build linux

package governancev069

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestVoteReplayLocalDurability(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private-votes")
	store, err := OpenAdvisoryVoteReplayStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	signed := testSignedVote(t)
	first, err := store.ReserveVerifiedAdvisoryVote(signed, validIntent(), validContext())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAdvisoryVoteReplayStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = reopened.ReserveVerifiedAdvisoryVote(signed, validIntent(), validContext())
	if !errors.Is(err, ErrAdvisoryVoteAlreadyReserved) {
		t.Fatalf("restart replay accepted: %v", err)
	}

	changed := signed.Vote
	changed.Choice = VoteNo
	changed.Nonce++
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	modified, err := SignAdvisoryVote(changed, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := VoteReplayKey(modified.Vote)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("nonce/choice evaded replay key")
	}
	_, err = reopened.ReserveVerifiedAdvisoryVote(modified, validIntent(), validContext())
	if !errors.Is(err, ErrAdvisoryVoteAlreadyReserved) {
		t.Fatalf("changed vote accepted: %v", err)
	}

	otherVote, otherKey := testVote(t, 8)
	otherSigned, err := SignAdvisoryVote(otherVote, otherKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.ReserveVerifiedAdvisoryVote(otherSigned, validIntent(), validContext()); err != nil {
		t.Fatalf("other voter should be distinct: %v", err)
	}

	tampered := otherSigned
	tampered.SignatureHex = strings.Repeat("0", 128)
	if _, err := reopened.ReserveVerifiedAdvisoryVote(tampered, validIntent(), validContext()); err == nil {
		t.Fatal("invalid signature wrote reservation")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want two reservations; got %d", len(files))
	}
	data, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["replay_key"] == "" || parsed["vote_sha256"] == "" {
		t.Fatal("missing record bindings")
	}
}

func TestVoteReplayConcurrentGoroutines(t *testing.T) {
	store, err := OpenAdvisoryVoteReplayStore(filepath.Join(t.TempDir(), "private-votes"))
	if err != nil {
		t.Fatal(err)
	}
	signed := testSignedVote(t)
	const n = 48
	var group sync.WaitGroup
	errorsChannel := make(chan error, n)
	for i := 0; i < n; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := store.ReserveVerifiedAdvisoryVote(signed, validIntent(), validContext())
			errorsChannel <- err
		}()
	}
	group.Wait()
	close(errorsChannel)
	accepted, duplicates := 0, 0
	for err := range errorsChannel {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrAdvisoryVoteAlreadyReserved):
			duplicates++
		default:
			t.Fatalf("unexpected admission error: %v", err)
		}
	}
	if accepted != 1 || duplicates != n-1 {
		t.Fatalf("admitted %d duplicate %d", accepted, duplicates)
	}
}

func TestVoteReplayRejectsUntrustedDirectories(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenAdvisoryVoteReplayStore("relative"); err == nil {
		t.Fatal("relative store path accepted")
	}
	// t.TempDir uses mode 0700 on Linux: private directories must be accepted.
	if _, err := OpenAdvisoryVoteReplayStore(dir); err != nil {
		t.Fatalf("private 0700 temp dir rejected: %v", err)
	}

	// Create and explicitly chmod a non-private directory: umask 077 can
	// otherwise turn Mkdir(..., 0755) into mode 0700 and mask this test.
	unsafe := filepath.Join(dir, "shared")
	if err := os.Mkdir(unsafe, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsafe, 0755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(unsafe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("test setup: want 0755, got %04o", info.Mode().Perm())
	}
	if _, err := OpenAdvisoryVoteReplayStore(unsafe); err == nil {
		t.Fatal("0755 dir accepted")
	}

	// Point this symlink to a valid private directory so a rejection must
	// come from the symlink guard, not from the target permissions.
	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(dir, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAdvisoryVoteReplayStore(symlink); err == nil {
		t.Fatal("symlink dir accepted")
	}
}

type processFixture struct {
	Signed   SignedAdvisoryVote `json:"signed"`
	Proposal ProposalIntent     `json:"proposal"`
	Context  Context            `json:"context"`
	Dir      string             `json:"dir"`
}

func TestVoteReplayChildProcess(t *testing.T) {
	if os.Getenv("PRISM_V069_CHILD") != "1" {
		return
	}
	contents, err := os.ReadFile(os.Getenv("PRISM_V069_FIXTURE"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture processFixture
	if err := json.Unmarshal(contents, &fixture); err != nil {
		t.Fatal(err)
	}
	store, err := OpenAdvisoryVoteReplayStore(fixture.Dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ReserveVerifiedAdvisoryVote(fixture.Signed, fixture.Proposal, fixture.Context)
	if err == nil {
		fmt.Println("RESULT:ACCEPTED")
		return
	}
	if errors.Is(err, ErrAdvisoryVoteAlreadyReserved) {
		fmt.Println("RESULT:DUPLICATE")
		return
	}
	t.Fatal(err)
}

func TestVoteReplayMultiprocess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "votes")
	if _, err := OpenAdvisoryVoteReplayStore(dir); err != nil {
		t.Fatal(err)
	}
	fixtureFile := filepath.Join(t.TempDir(), "fixture.json")
	fixture := processFixture{Signed: testSignedVote(t), Proposal: validIntent(), Context: validContext(), Dir: dir}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixtureFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	const n = 8
	outputs := make(chan string, n)
	failures := make(chan error, n)
	var group sync.WaitGroup
	for i := 0; i < n; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestVoteReplayChildProcess$", "-test.v")
			cmd.Env = append(os.Environ(), "PRISM_V069_CHILD=1", "PRISM_V069_FIXTURE="+fixtureFile)
			out, err := cmd.CombinedOutput()
			if err != nil {
				failures <- fmt.Errorf("child: %w: %s", err, string(out))
				return
			}
			outputs <- string(out)
		}()
	}
	group.Wait()
	close(outputs)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	accepted, duplicates := 0, 0
	for out := range outputs {
		if strings.Contains(out, "RESULT:ACCEPTED") {
			accepted++
		} else if strings.Contains(out, "RESULT:DUPLICATE") {
			duplicates++
		} else {
			t.Fatalf("unexpected child output: %s", out)
		}
	}
	if accepted != 1 || duplicates != n-1 {
		t.Fatalf("multiprocess accepted %d duplicate %d", accepted, duplicates)
	}
}
