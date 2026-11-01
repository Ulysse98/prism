//go:build windows

package crosschain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Windows must not claim Linux-only local replay persistence protection.
func TestLocalReplayV067WindowsExplicitlyFailsClosed(t *testing.T) {
	base := t.TempDir()
	store, err := NewLocalReplayStoreV067(base)
	if store != nil || !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("Windows replay store must fail closed; store=%v err=%v", store, err)
	}
	namespace := filepath.Join(base, localReplayDirectoryV067)
	if _, err := os.Lstat(namespace); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported Windows must not create replay namespace: %v", err)
	}
	var nilStore *LocalReplayStoreV067
	if _, err := nilStore.reserveProofV067(localReplaySampleIntentV067()); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("nil replay store must refuse reservations: %v", err)
	}
}
