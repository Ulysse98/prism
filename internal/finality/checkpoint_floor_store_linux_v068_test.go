//go:build linux

package finality

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func v068FreshFloorDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private-floor")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestV068FloorStoreInitReadAndFailClosedRecovery(t *testing.T) {
	_, _, _, _, floor, witness := v068TrustFloorFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCheckpointFloorStoreV068(dir, witness)
	if err != nil || got != floor {
		t.Fatalf("read mismatch: %v", err)
	}
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err == nil {
		t.Fatal("reinit permitted")
	}
	old := witness
	old.Revision--
	if _, err := ReadCheckpointFloorStoreV068(dir, old); err == nil {
		t.Fatal("old external witness accepted")
	}
	if err := os.Remove(filepath.Join(dir, floorStoreFileV068)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, witness); err == nil {
		t.Fatal("missing record accepted")
	}
}

func TestV068FloorStorePrepareRecoverAndCommitExternalWitness(t *testing.T) {
	bc, validators, nextSigned, policy, floor, witness := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
		t.Fatal(err)
	}
	next, err := PrepareCheckpointFloorAdvanceV068(dir, bc, validators, nextSigned, policy, witness)
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != witness.Revision+1 {
		t.Fatal("wrong next revision")
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, witness); err == nil {
		t.Fatal("pending state exposed as current")
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, next); err == nil {
		t.Fatal("premature external advance exposed")
	}
	if err := CommitPreparedCheckpointFloorV068(dir, witness); err == nil {
		t.Fatal("old external witness committed pending")
	}
	if err := CommitPreparedCheckpointFloorV068(dir, next); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCheckpointFloorStoreV068(dir, next)
	if err != nil || got.Anchor.ParentHeight != 1 || got.Revision != next.Revision {
		t.Fatalf("commit/read mismatch: %v", err)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, witness); err == nil {
		t.Fatal("old witness after commit accepted")
	}
	if err := CommitPreparedCheckpointFloorV068(dir, next); err == nil {
		t.Fatal("double commit accepted")
	}
}

func TestV068FloorStoreRejectsInvalidCheckpointWithoutPending(t *testing.T) {
	bc, validators, nextSigned, policy, floor, witness := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
		t.Fatal(err)
	}
	nextSigned.SignatureHex = strings.Repeat("00", 64)
	if _, err := PrepareCheckpointFloorAdvanceV068(dir, bc, validators, nextSigned, policy, witness); err == nil {
		t.Fatal("forged checkpoint staged")
	}
	if got, err := ReadCheckpointFloorStoreV068(dir, witness); err != nil || got != floor {
		t.Fatalf("invalid stage damaged current floor: %v", err)
	}
}

func TestV068FloorStoreFaultInjectionBeforeAndAfterRename(t *testing.T) {
	bc, validators, nextSigned, policy, floor, witness := v068ForwardCheckpointFixture(t)
	for _, stage := range []string{"before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			dir := v068FreshFloorDir(t)
			if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
				t.Fatal(err)
			}
			_, err := prepareCheckpointFloorHookV068(dir, bc, validators, nextSigned, policy, witness, func(s string) error {
				if s == stage {
					return errors.New("injected fault")
				}
				return nil
			})
			if err == nil {
				t.Fatal("fault not propagated")
			}
			_, readErr := ReadCheckpointFloorStoreV068(dir, witness)
			if stage == "before-rename" && readErr != nil {
				t.Fatalf("pre-rename should preserve current: %v", readErr)
			}
			if stage == "after-rename" && readErr == nil {
				t.Fatal("post-rename pending state must freeze admission")
			}
		})
	}
}

func TestV068FloorStoreConcurrentPrepareSingleWinner(t *testing.T) {
	bc, validators, nextSigned, policy, floor, witness := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
		t.Fatal(err)
	}
	const workers = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := PrepareCheckpointFloorAdvanceV068(dir, bc, validators, nextSigned, policy, witness)
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("expected one successful prepare, got %d", success)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, witness); err == nil {
		t.Fatal("pending state unexpectedly readable")
	}
}

func TestV068FloorStoreRejectsCorruptionAndSymlinks(t *testing.T) {
	_, _, _, _, floor, witness := v068TrustFloorFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, floorStoreFileV068)
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, witness); err == nil {
		t.Fatal("truncated record accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "fake")
	if err := os.WriteFile(target, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, witness); err == nil {
		t.Fatal("symlink accepted")
	}
}
