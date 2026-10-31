//go:build linux

package finality

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"prism/internal/wallet"
	"strings"
	"sync"
	"testing"
)

func durableFixtureV065() durableBindingV065 {
	return durableBindingV065{ChainID: "prism-durable-lab", GenesisHash: strings.Repeat("a", 64), ValidatorSetHash: strings.Repeat("b", 64), SignerAddress: "prism_test", Height: 3}
}
func durableDirV065(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "safety")
	if err := os.Mkdir(d, 0700); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDurableV065InitRecoverAndSameProcessConflict(t *testing.T) {
	d, b := durableDirV065(t), durableFixtureV065()
	if _, err := readDurableV065(d, b); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing state must fail closed: %v", err)
	}
	if err := initDurableV065(d, b); err != nil {
		t.Fatal(err)
	}
	if err := initDurableV065(d, b); err == nil {
		t.Fatal("re-initialization accepted")
	}
	h := strings.Repeat("c", 64)
	_, err := transactDurableV065(d, b, func(r *durableBFTRecordV065) error {
		r.Snapshot.Round = 1
		r.Snapshot.Step = BFTPrecommitV065
		r.Snapshot.HasLock = true
		r.Snapshot.LockedRound = 1
		r.Snapshot.LockedBlock = h
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := readDurableV065(d, b)
	if err != nil || r.Revision != 2 || !r.Snapshot.HasLock || r.Snapshot.LockedBlock != h {
		t.Fatalf("lost lock: %+v %v", r, err)
	}
	_, err = transactDurableV065(d, b, func(r *durableBFTRecordV065) error { r.Snapshot.HasLock = false; return nil })
	if err == nil {
		t.Fatal("inconsistent state accepted")
	}
}

func TestDurableV065FailureBeforeAndAfterRename(t *testing.T) {
	d, b := durableDirV065(t), durableFixtureV065()
	if err := initDurableV065(d, b); err != nil {
		t.Fatal(err)
	}
	change := func(r *durableBFTRecordV065) error { r.Snapshot.Round++; return nil }
	_, err := transactDurableHookV065(d, b, change, func(stage string) error {
		if stage == "before-rename" {
			return errors.New("injected pre-rename failure")
		}
		return nil
	})
	if err == nil {
		t.Fatal("pre-rename fault ignored")
	}
	r, err := readDurableV065(d, b)
	if err != nil || r.Revision != 1 || r.Snapshot.Round != 0 {
		t.Fatalf("pre-rename mutation persisted: %+v %v", r, err)
	}
	_, err = transactDurableHookV065(d, b, change, func(stage string) error {
		if stage == "after-rename" {
			return errors.New("injected post-rename failure")
		}
		return nil
	})
	if err == nil {
		t.Fatal("post-rename fault ignored")
	}
	// Visibility after rename does NOT establish durability after real power loss.
	r, err = readDurableV065(d, b)
	if err != nil || r.Revision != 2 || r.Snapshot.Round != 1 {
		t.Fatalf("post-rename process recovery: %+v %v", r, err)
	}
}

func TestDurableV065CorruptionWrongIdentityAndSymlink(t *testing.T) {
	d, b := durableDirV065(t), durableFixtureV065()
	if err := initDurableV065(d, b); err != nil {
		t.Fatal(err)
	}
	wrong := b
	wrong.GenesisHash = strings.Repeat("d", 64)
	if _, err := readDurableV065(d, wrong); err == nil {
		t.Fatal("wrong network accepted")
	}
	if _, err := transactDurableV065(d, b, func(r *durableBFTRecordV065) error { r.Binding = wrong; return nil }); err == nil {
		t.Fatal("binding modification accepted")
	}
	path := filepath.Join(d, durableStoreFilenameV065)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDurableV065(d, b); err == nil {
		t.Fatal("unsafe permissions accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDurableV065(d, b); err == nil {
		t.Fatal("corrupt state accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := readDurableV065(d, b); err == nil {
		t.Fatal("lost state was reset")
	}
	if err := os.Symlink("/etc/passwd", path); err != nil {
		t.Fatal(err)
	}
	if _, err := readDurableV065(d, b); err == nil {
		t.Fatal("symlink followed")
	}
}

func TestDurableV065VotesBindToSignerAndPreventDuplicateSlot(t *testing.T) {
	w, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	d, b := durableDirV065(t), durableFixtureV065()
	b.SignerAddress = w.Address
	if err := initDurableV065(d, b); err != nil {
		t.Fatal(err)
	}
	ctx := VoteContext{ChainID: b.ChainID, GenesisHash: b.GenesisHash, Height: b.Height, Round: 0, Step: stepPrevote, BlockHash: strings.Repeat("c", 64), ValidatorSetHash: b.ValidatorSetHash}
	vote, err := SignVote(ctx, w.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = transactDurableV065(d, b, func(r *durableBFTRecordV065) error { r.Votes = append(r.Votes, vote); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transactDurableV065(d, b, func(r *durableBFTRecordV065) error { r.Votes = append(r.Votes, vote); return nil }); err == nil {
		t.Fatal("duplicate signing slot accepted")
	}
	if _, err := transactDurableV065(d, b, func(r *durableBFTRecordV065) error { r.Votes[0].Signature = "invalid"; return nil }); err == nil {
		t.Fatal("forged signature accepted")
	}
}

func TestDurableV065ChildProcess(t *testing.T) {
	d := os.Getenv("PRISM_DURABLE_V065_TEST_CHILD")
	if d == "" {
		return
	}
	b := durableFixtureV065()
	if _, err := transactDurableV065(d, b, func(r *durableBFTRecordV065) error { r.Snapshot.Round++; return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestDurableV065MultiProcessSerialization(t *testing.T) {
	d, b := durableDirV065(t), durableFixtureV065()
	if err := initDurableV065(d, b); err != nil {
		t.Fatal(err)
	}
	const n = 12
	failures := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestDurableV065ChildProcess$")
			cmd.Env = append(os.Environ(), "PRISM_DURABLE_V065_TEST_CHILD="+d)
			out, err := cmd.CombinedOutput()
			if err != nil {
				failures <- string(out) + err.Error()
			}
		}()
	}
	wg.Wait()
	close(failures)
	for f := range failures {
		t.Error(f)
	}
	if t.Failed() {
		return
	}
	r, err := readDurableV065(d, b)
	if err != nil {
		t.Fatal(err)
	}
	if r.Revision != n+1 || r.Snapshot.Round != n {
		t.Fatalf("lost interprocess update: revision=%d round=%d", r.Revision, r.Snapshot.Round)
	}
}
