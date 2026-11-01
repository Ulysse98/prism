package crosschain

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func localReplaySampleIntentV067() AnchorIntentV067 {
	return AnchorIntentV067{
		Version:             AnchorIntentVersionV067,
		SourceChainID:       "prism-local-test",
		SourceGenesisHash:   strings.Repeat("a", 64),
		SourceBlockHeight:   11,
		SourceBlockHash:     strings.Repeat("b", 64),
		JobID:               strings.Repeat("c", 64),
		ProofHash:           strings.Repeat("d", 64),
		DestinationChainID:  "arbitrum-sepolia",
		DestinationRegistry: "registry-local-test",
		Nonce:               7,
	}
}

func TestLocalReplayV067ReserveRestartAndDestinationBypass(t *testing.T) {
	base := t.TempDir()
	store, err := NewLocalReplayStoreV067(base)
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	record, err := store.reserveProofV067(intent)
	if err != nil {
		t.Fatal(err)
	}
	if record.ProofKey == "" || record.IntentDigest == "" {
		t.Fatal("missing committed reservation identities")
	}
	audited, err := store.InspectProofV067(intent)
	if err != nil || audited.ProofKey != record.ProofKey {
		t.Fatalf("inspect: %+v %v", audited, err)
	}

	if _, err := store.reserveProofV067(intent); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("identical replay must fail: %v", err)
	}
	restarted, err := NewLocalReplayStoreV067(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.reserveProofV067(intent); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("restart replay must fail: %v", err)
	}
	changed := intent
	changed.Nonce++
	changed.DestinationChainID = "monad-testnet"
	changed.DestinationRegistry = "another-registry"
	changed.SourceBlockHeight++
	if _, err := restarted.reserveProofV067(changed); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("destination/nonce/block bypass must fail: %v", err)
	}
	if _, err := restarted.InspectProofV067(changed); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("different intent must not be accepted as original audit record: %v", err)
	}
	differentProof := intent
	differentProof.ProofHash = strings.Repeat("e", 64)
	if _, err := restarted.reserveProofV067(differentProof); err != nil {
		t.Fatalf("distinct proof should have its own slot: %v", err)
	}
	info, err := os.Stat(store.recordPath(record.ProofKey))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("reservation file must have private permissions: %v / %v", info, err)
	}
}

func TestLocalReplayV067ConcurrentIndependentStoreInstances(t *testing.T) {
	base := t.TempDir()
	const workers = 32
	var wg sync.WaitGroup
	outcomes := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store, err := NewLocalReplayStoreV067(base)
			if err == nil {
				_, err = store.reserveProofV067(localReplaySampleIntentV067())
			}
			outcomes <- err
		}()
	}
	wg.Wait()
	close(outcomes)
	wins, replayRejects := 0, 0
	for err := range outcomes {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrLocalReplayReservedV067):
			replayRejects++
		default:
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if wins != 1 || replayRejects != workers-1 {
		t.Fatalf("expected exactly one winner: wins=%d reject=%d", wins, replayRejects)
	}
}

func TestLocalReplayV067CorruptionAndTombstoneFailClosed(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	saved, err := store.reserveProofV067(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.recordPath(saved.ProofKey), []byte(`{"version":1,"incomplete":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InspectProofV067(intent); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("corrupted record must fail inspection: %v", err)
	}
	if _, err := store.reserveProofV067(intent); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("corrupted record must continue to block replay: %v", err)
	}
	second := intent
	second.ProofHash = strings.Repeat("e", 64)
	key, err := localProofKeyV067(second)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.recordPath(key), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.reserveProofV067(second); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("partial/crash tombstone must block replay: %v", err)
	}
	if _, err := store.InspectProofV067(second); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("partial record must fail inspection: %v", err)
	}
}

func TestLocalReplayV067InvalidIntentAndStorageFailure(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bad := localReplaySampleIntentV067()
	bad.Nonce = 0
	if _, err := store.reserveProofV067(bad); err == nil {
		t.Fatal("invalid intent accepted")
	}
	entries, err := os.ReadDir(store.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid intent occupied storage: %v / %v", entries, err)
	}
	if err := os.Remove(store.dir); err != nil {
		t.Fatal(err)
	}
	if _, err := store.reserveProofV067(localReplaySampleIntentV067()); err == nil {
		t.Fatal("missing data directory accepted")
	}
}

func TestLocalReplayV067SymlinkAndExtraJSONFailClosed(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	key, err := localProofKeyV067(intent)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "external.json")
	if err := os.WriteFile(destination, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(destination, store.recordPath(key)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.reserveProofV067(intent); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("symlink slot must block replay: %v", err)
	}
	if _, err := store.InspectProofV067(intent); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("symlink record must fail inspection: %v", err)
	}
	if err := os.Remove(store.recordPath(key)); err != nil {
		t.Fatal(err)
	}
	record, err := store.reserveProofV067(intent)
	if err != nil {
		t.Fatal(err)
	}
	path := store.recordPath(record.ProofKey)
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(bytes, []byte(`{"extra":true}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InspectProofV067(intent); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("trailing JSON must fail inspection: %v", err)
	}
}

// A separate OS process must observe the same durable one-shot reservation.
func TestLocalReplayV067CrossProcessRestart(t *testing.T) {
	base := t.TempDir()
	store, err := NewLocalReplayStoreV067(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.reserveProofV067(localReplaySampleIntentV067()); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLocalReplayV067ChildProcess$")
	cmd.Env = append(os.Environ(), "PRISM_V067_REPLAY_CHILD=1", "PRISM_V067_REPLAY_ROOT="+base)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cross-process replay test failed: %s: %v", output, err)
	}
}

func TestLocalReplayV067ChildProcess(t *testing.T) {
	if os.Getenv("PRISM_V067_REPLAY_CHILD") != "1" {
		return
	}
	store, err := NewLocalReplayStoreV067(os.Getenv("PRISM_V067_REPLAY_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.reserveProofV067(localReplaySampleIntentV067()); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("process replay must fail: %v", err)
	}
	if _, err := store.InspectProofV067(localReplaySampleIntentV067()); err != nil {
		t.Fatalf("cross-process audit read failed: %v", err)
	}
}
