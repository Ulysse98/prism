//go:build linux

package governancev069

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func recoveryFloorFixture(t *testing.T) (*AdvisoryVoteReplayStore, AdvisoryClosureRecoveryFloor, AdvisoryBallotClosure) {
	t.Helper()
	store, receipt, snapshot, snapPin, ballotPin, ballots, proposal, ctx := closeFixture(t)
	c, err := closeCall(store, receipt, snapshot, snapPin, ballotPin, ballots, proposal, ctx)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := BuildAdvisoryClosureRecoveryFloor(c, closureOperatorPin(), 4)
	if err != nil {
		t.Fatal(err)
	}
	return store, floor, c
}

func TestExternalRecoveryFloorReadOnlyAndReopen(t *testing.T) {
	store, floor, sealed := recoveryFloorFixture(t)
	if err := ValidateAdvisoryClosureRecoveryFloor(floor); err != nil {
		t.Fatal(err)
	}
	if err := VerifyAdvisoryClosureAgainstFloor(sealed, floor); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAdvisoryVoteReplayStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := InspectLocalAdvisoryClosureAgainstFloor(reopened, floor)
	if err != nil || result.Status != AdvisoryFloorRecoveryVerified || result.ClosureSHA256 != sealed.ClosureSHA256 || result.FloorSHA256 != floor.FloorSHA256 {
		t.Fatalf("unexpected externally pinned recovery: %+v, %v", result, err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := InspectLocalAdvisoryClosureAgainstFloor(reopened, floor); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	// Freeze a deterministic digest for the fixed existing fixture.
	const goldenFloor = "c98ea994589060155ef0fd1ad88dcd4f5304964ead694c3fb69e198de9f07589"
	if floor.FloorSHA256 != goldenFloor {
		t.Fatalf("canonical recovery floor vector changed: got %s expected %s", floor.FloorSHA256, goldenFloor)
	}
}

func TestExternalRecoveryFloorRejectsTamperAndMissingMarker(t *testing.T) {
	store, floor, sealed := recoveryFloorFixture(t)
	marker := filepath.Join(store.dir, localBallotClosureFilename)
	bads := []struct {
		name string
		f    func(*AdvisoryClosureRecoveryFloor)
	}{
		{"wrong authority", func(f *AdvisoryClosureRecoveryFloor) {
			f.AuthorityPublicKeyHex = hex.EncodeToString(foreignClosureKey().Public().(ed25519.PublicKey))
		}},
		{"wrong epoch", func(f *AdvisoryClosureRecoveryFloor) { f.AuthorityEpoch++ }},
		{"wrong height", func(f *AdvisoryClosureRecoveryFloor) { f.ClosedAtHeight++ }},
		{"wrong marker", func(f *AdvisoryClosureRecoveryFloor) { f.MarkerSHA256 = strings.Repeat("0", 64) }},
		{"wrong proposal", func(f *AdvisoryClosureRecoveryFloor) { f.ProposalSHA256 = strings.Repeat("0", 64) }},
		{"bad commitment", func(f *AdvisoryClosureRecoveryFloor) { f.FloorSHA256 = strings.Repeat("0", 64) }},
	}
	for _, test := range bads {
		t.Run(test.name, func(t *testing.T) {
			changed := floor
			test.f(&changed)
			if _, err := InspectLocalAdvisoryClosureAgainstFloor(store, changed); err == nil {
				t.Fatal("tampered floor accepted")
			}
		})
	}
	altered := sealed
	altered.Authorization, _ = SignAdvisoryClosureAuthorization(altered, 999, closureOperatorKey())
	// Same closure body and trusted signer, different signed marker bytes: pinned
	// marker hash must reject a replacement record with a fresh nonce.
	if err := VerifyAdvisoryClosureAgainstFloor(altered, floor); err == nil {
		t.Fatal("rewritten signed marker accepted")
	}
	payload, err := json.Marshal(altered)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, append(payload, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if result, err := InspectLocalAdvisoryClosureAgainstFloor(store, floor); err == nil || result.Status != AdvisoryFloorRecoveryRejected {
		t.Fatalf("tampered signed marker passed: %+v %v", result, err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if result, err := InspectLocalAdvisoryClosureAgainstFloor(store, floor); err == nil || result.Status != AdvisoryFloorRecoveryRejected {
		t.Fatalf("missing marker passed: %+v %v", result, err)
	}
	// A missing pin must fail, not silently fall back to the marker's own key.
	if _, err := InspectLocalAdvisoryClosureAgainstFloor(store, AdvisoryClosureRecoveryFloor{}); err == nil {
		t.Fatal("empty floor accepted")
	}
}

func TestExternalRecoveryFloorNoFalseGlobalFinality(t *testing.T) {
	store, floor, sealed := recoveryFloorFixture(t)
	// A second proposal with a different digest must be rejected even when
	// signed by the same operator key.
	changed := sealed
	changed.ProposalSHA256 = strings.Repeat("0", 64)
	digest := localBallotClosureCommitment(changed)
	changed.ClosureSHA256 = hex.EncodeToString(digest[:])
	changed.Authorization, _ = SignAdvisoryClosureAuthorization(changed, 19, closureOperatorKey())
	if err := VerifyAdvisoryClosureAgainstFloor(changed, floor); err == nil {
		t.Fatal("different proposal accepted")
	}
	original, err := os.ReadFile(filepath.Join(store.dir, localBallotClosureFilename))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(original)
	if hex.EncodeToString(sum[:]) != floor.MarkerSHA256 {
		t.Fatal("pinned hash not exact on-disk marker bytes")
	}
}
