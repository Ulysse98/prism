package crosschain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// This subprocess deliberately occupies a slot and exits without calling
// Sync, Close, or returning through the test framework. It models process
// interruption while a partial record exists; it DOES NOT simulate power loss.
func TestLocalReplayRecoveryV067AbortChild(t *testing.T) {
	if os.Getenv("PRISM_V067_RECOVERY_CHILD") != "abort" {
		return
	}
	base := os.Getenv("PRISM_V067_RECOVERY_ROOT")
	store, err := NewLocalReplayStoreV067(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
	key, err := localProofKeyV067(localReplaySampleIntentV067())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(24)
	}
	fd, err := os.OpenFile(store.recordPath(key), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(25)
	}
	if _, err := fd.Write([]byte(`{"version":1,"incomplete":`)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(26)
	}
	// Deliberately skip fd.Sync(), fd.Close() and directory sync.
	os.Exit(0)
}

func TestLocalReplayRecoveryV067InterruptedWriterBlocksRetry(t *testing.T) {
	base := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLocalReplayRecoveryV067AbortChild$")
	cmd.Env = append(os.Environ(), "PRISM_V067_RECOVERY_CHILD=abort", "PRISM_V067_RECOVERY_ROOT="+base)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("aborted writer subprocess failed: %s: %v", output, err)
	}
	store, err := NewLocalReplayStoreV067(base)
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	if _, err := store.reserveProofV067(intent); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("partial slot should block restart/retry: %v", err)
	}
	if _, err := store.InspectProofV067(intent); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("partial slot must not pass inspection: %v", err)
	}
	again, err := NewLocalReplayStoreV067(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := again.reserveProofV067(intent); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("partial slot must remain reserved after new instance: %v", err)
	}
}

// Each child is a separate OS process, not just a goroutine or store instance.
func TestLocalReplayRecoveryV067ContenderChild(t *testing.T) {
	if os.Getenv("PRISM_V067_RECOVERY_CHILD") != "contender" {
		return
	}
	store, err := NewLocalReplayStoreV067(os.Getenv("PRISM_V067_RECOVERY_ROOT"))
	if err != nil {
		t.Fatalf("open contender store: %v", err)
	}
	_, err = store.reserveProofV067(localReplaySampleIntentV067())
	switch {
	case err == nil:
		fmt.Println("PRISM_V067_RECOVERY_WIN")
	case errors.Is(err, ErrLocalReplayReservedV067):
		fmt.Println("PRISM_V067_RECOVERY_REPLAY")
	default:
		t.Fatalf("unexpected contender failure: %v", err)
	}
}

func TestLocalReplayRecoveryV067ConcurrentProcesses(t *testing.T) {
	const contenders = 12
	base := t.TempDir()
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, contenders)
	errorsFound := make(chan error, contenders)
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			cmd := exec.Command(os.Args[0], "-test.run=^TestLocalReplayRecoveryV067ContenderChild$")
			cmd.Env = append(os.Environ(), "PRISM_V067_RECOVERY_CHILD=contender", "PRISM_V067_RECOVERY_ROOT="+base)
			output, err := cmd.CombinedOutput()
			if err != nil {
				errorsFound <- fmt.Errorf("contender failed: %s: %w", output, err)
				return
			}
			s := string(output)
			switch {
			case strings.Contains(s, "PRISM_V067_RECOVERY_WIN"):
				results <- "win"
			case strings.Contains(s, "PRISM_V067_RECOVERY_REPLAY"):
				results <- "replay"
			default:
				errorsFound <- fmt.Errorf("unrecognized contender result: %s", output)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	wins, replays := 0, 0
	for result := range results {
		if result == "win" {
			wins++
		} else {
			replays++
		}
	}
	if wins != 1 || replays != contenders-1 {
		t.Fatalf("expected one process winner, got wins=%d replay=%d", wins, replays)
	}
	store, err := NewLocalReplayStoreV067(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InspectProofV067(localReplaySampleIntentV067()); err != nil {
		t.Fatalf("winning process should leave inspectable durable record: %v", err)
	}
}

func TestLocalReplayRecoveryV067UnsafeNamespaceRejected(t *testing.T) {
	t.Run("loose directory permissions", func(t *testing.T) {
		base := t.TempDir()
		namespace := filepath.Join(base, localReplayDirectoryV067)
		if err := os.Mkdir(namespace, 0755); err != nil {
			t.Fatal(err)
		}
		// os.Mkdir applies the process umask. Explicitly create the unsafe
		// permissions that this negative test must exercise even under umask 077.
		if err := os.Chmod(namespace, 0755); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(namespace)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0755 {
			t.Fatalf("unsafe namespace fixture has mode %04o; want 0755", got)
		}
		if _, err := NewLocalReplayStoreV067(base); !errors.Is(err, ErrLocalReplayInvalidV067) {
			t.Fatalf("world/traverse-readable namespace accepted: %v", err)
		}
	})
	t.Run("symlinked namespace", func(t *testing.T) {
		base := t.TempDir()
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(base, localReplayDirectoryV067)); err != nil {
			t.Fatal(err)
		}
		if _, err := NewLocalReplayStoreV067(base); !errors.Is(err, ErrLocalReplayInvalidV067) {
			t.Fatalf("symlink namespace accepted: %v", err)
		}
	})
}
