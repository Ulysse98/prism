package crosschain

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestLocalAnchorAdmissionV067PreflightFailureLeavesSlotEmpty(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	sentinel := errors.New("evidence rejected")
	calls := 0
	_, err = admitLocalAnchorCandidateWithPreflightV067(store, intent, func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) || calls != 1 {
		t.Fatalf("preflight rejection lost: err=%v calls=%d", err, calls)
	}
	entries, err := os.ReadDir(store.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed verification must never occupy a proof slot: %v / %v", entries, err)
	}
}

func TestLocalAnchorAdmissionV067MalformedIntentSkipsPreflight(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bad := localReplaySampleIntentV067()
	bad.Nonce = 0
	invoked := false
	if _, err := admitLocalAnchorCandidateWithPreflightV067(store, bad, func() error {
		invoked = true
		return nil
	}); err == nil || invoked {
		t.Fatalf("malformed intent must be rejected before verification: %v, called=%v", err, invoked)
	}
	entries, err := os.ReadDir(store.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid intent occupied slot: %v / %v", entries, err)
	}
}

func TestLocalAnchorAdmissionV067PositiveSeamAndReplay(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	// This mocks ONLY the verified-then-reserve control-flow boundary.
	// It does NOT claim a real positive PoUW/checkpoint verification.
	record, err := admitLocalAnchorCandidateWithPreflightV067(store, intent, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if record.ProofKey == "" || record.IntentDigest == "" {
		t.Fatal("missing identities")
	}
	if _, err := store.InspectProofV067(intent); err != nil {
		t.Fatal(err)
	}
	altered := intent
	altered.Nonce++
	altered.DestinationChainID = "monad-testnet"
	altered.DestinationRegistry = "other-registry"
	if _, err := admitLocalAnchorCandidateWithPreflightV067(store, altered, func() error { return nil }); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("cross-destination replay succeeded: %v", err)
	}
	if !ErrLocalAnchorAdmissionReplayV067(ErrLocalReplayReservedV067) {
		t.Fatal("lost replay sentinel")
	}
}

func TestLocalAnchorAdmissionV067MissingGateFailsClosed(t *testing.T) {
	intent := localReplaySampleIntentV067()
	if _, err := admitLocalAnchorCandidateWithPreflightV067(nil, intent, func() error { return nil }); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("nil store accepted: %v", err)
	}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitLocalAnchorCandidateWithPreflightV067(store, intent, nil); !errors.Is(err, ErrLocalReplayInvalidV067) {
		t.Fatalf("missing preflight accepted: %v", err)
	}
	if err := os.Remove(store.dir); err != nil {
		t.Fatal(err)
	}
	if _, err := admitLocalAnchorCandidateWithPreflightV067(store, intent, func() error { return nil }); err == nil {
		t.Fatal("unavailable store accepted")
	}
}

func TestLocalAnchorAdmissionV067IndependentConcurrentCalls(t *testing.T) {
	base := t.TempDir()
	intent := localReplaySampleIntentV067()
	const contenders = 24
	var wg sync.WaitGroup
	out := make(chan error, contenders)
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store, err := NewLocalReplayStoreV067(base)
			if err == nil {
				_, err = admitLocalAnchorCandidateWithPreflightV067(store, intent, func() error { return nil })
			}
			out <- err
		}()
	}
	wg.Wait()
	close(out)
	winners, replays := 0, 0
	for err := range out {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrLocalReplayReservedV067):
			replays++
		default:
			t.Fatalf("unexpected concurrency error: %v", err)
		}
	}
	if winners != 1 || replays != contenders-1 {
		t.Fatalf("expected exactly 1 winner and %d replays, got %d/%d", contenders-1, winners, replays)
	}
}

func TestLocalAnchorAdmissionV067RealVerifierRejectsInvalidCandidate(t *testing.T) {
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	intent := localReplaySampleIntentV067()
	candidate := LocalAnchorCandidateV067{Intent: intent}
	policy := AnchorProofPolicyV067{
		SourceChainID:       intent.SourceChainID,
		SourceGenesisHash:   intent.SourceGenesisHash,
		ApprovedArtifactSHA: strings.Repeat("e", 64),
		ApprovedVerifier:    "trusted-test-verifier",
		DestinationChainID:  intent.DestinationChainID,
		DestinationRegistry: intent.DestinationRegistry,
	}
	checkpoint := PinnedSourceCheckpointV067{
		SourceChainID:       intent.SourceChainID,
		SourceGenesisHash:   intent.SourceGenesisHash,
		CheckpointHeight:    intent.SourceBlockHeight + 1,
		CheckpointBlockHash: strings.Repeat("f", 64),
		MinSuccessorBlocks:  1,
	}
	// Real verifier: nil chain / validators must fail even if the supplied
	// operator pins and intent are structurally well formed.
	if _, err := admitLocalAnchorCandidateV067(store, candidate, policy, checkpoint, nil, nil); err == nil {
		t.Fatal("unverified source candidate admitted")
	}
	entries, err := os.ReadDir(store.dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("real verifier refusal occupied storage: %v / %v", entries, err)
	}
	if _, err := admitLocalAnchorCandidateWithPreflightV067(store, intent, func() error { return fmt.Errorf("reject proof: %w", errors.New("invalid signed proof")) }); err == nil {
		t.Fatal("explicit invalid proof unexpectedly admitted")
	}
}
