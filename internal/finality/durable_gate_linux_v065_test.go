//go:build linux

package finality

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"prism/internal/consensus"
)

func gateFixtureV065(t *testing.T) (lockFixtureV065, string, durableBindingV065) {
	t.Helper()
	f := fixtureLockV065(t)
	d := durableDirV065(t)
	if err := InitDurableGateV065(d, "prism-lock-lab", f.genesis, 3, f.validators, f.keys[0]); err != nil {
		t.Fatal(err)
	}
	b, err := durableGateBindingV065("prism-lock-lab", f.genesis, 3, f.validators, f.keys[0])
	if err != nil {
		t.Fatal(err)
	}
	return f, d, b
}

func gatePrevoteV065(d string, f lockFixtureV065, proposal string, valid bool, pol *Certificate) (SignedVote, error) {
	return SignDurableGatePrevoteV065(d, "prism-lock-lab", f.genesis, 3, f.validators, f.keys[0], proposal, valid, pol)
}
func gatePrecommitV065(d string, f lockFixtureV065, cert *Certificate, valid bool) (SignedVote, error) {
	return SignDurableGatePrecommitV065(d, "prism-lock-lab", f.genesis, 3, f.validators, f.keys[0], cert, valid)
}
func gateAdvanceV065(d string, f lockFixtureV065, next uint64) error {
	return AdvanceDurableGateRoundV065(d, "prism-lock-lab", f.genesis, 3, f.validators, f.keys[0], next)
}
func gateReadV065(t *testing.T, d string, b durableBindingV065, f lockFixtureV065) durableBFTRecordV065 {
	t.Helper()
	r, err := readDurableV065(d, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replayDurableGateV065(r, b, f.validators); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestGateV065RoundLockNILAndRestart(t *testing.T) {
	f, d, b := gateFixtureV065(t)
	a, err := gatePrevoteV065(d, f, f.blockA, true, nil)
	if err != nil || a.Context.Nil || a.Context.BlockHash != f.blockA {
		t.Fatalf("first prevote %v", err)
	}
	again, err := gatePrevoteV065(d, f, f.blockA, true, nil)
	if err != nil || a != again {
		t.Fatal("idempotent replay failed", err)
	}
	if r := gateReadV065(t, d, b, f); r.Revision != 2 || len(r.Events) != 1 {
		t.Fatal("idempotent retry mutated state")
	}
	if _, err := gatePrevoteV065(d, f, f.blockB, true, nil); err == nil {
		t.Fatal("double voted")
	}
	certA := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	p, err := gatePrecommitV065(d, f, &certA, true)
	if err != nil || p.Context.BlockHash != f.blockA {
		t.Fatalf("precommit A: %v", err)
	}
	r := gateReadV065(t, d, b, f)
	if !r.Snapshot.HasLock || r.Snapshot.LockedBlock != f.blockA {
		t.Fatal("missing durable lock")
	}
	if err := gateAdvanceV065(d, f, 1); err != nil {
		t.Fatal(err)
	}
	blocked, err := gatePrevoteV065(d, f, f.blockB, true, nil)
	if err != nil || blocked.Context.BlockHash != f.blockA {
		t.Fatalf("lock bypassed after restart: %v", err)
	}
	nilCert := nilLabCertificateV065(t, f, stepPrevote, 1, 0, 1)
	vote, err := gatePrecommitV065(d, f, &nilCert, false)
	if err != nil || !vote.Context.Nil {
		t.Fatalf("expected NIL: %v", err)
	}
	r = gateReadV065(t, d, b, f)
	if r.Snapshot.HasLock || r.Snapshot.Round != 1 {
		t.Fatal("NIL did not persist unlock")
	}
	if err := gateAdvanceV065(d, f, 2); err != nil {
		t.Fatal(err)
	}
	next, err := gatePrevoteV065(d, f, f.blockB, true, nil)
	if err != nil || next.Context.BlockHash != f.blockB {
		t.Fatalf("post-unlock prevote failed: %v", err)
	}
	if len(gateReadV065(t, d, b, f).Votes) != 5 {
		t.Fatal("missing vote history")
	}
}

func TestGateV065RejectTamperedStateWithoutTransitionProvenance(t *testing.T) {
	f, d, b := gateFixtureV065(t)
	if _, err := gatePrevoteV065(d, f, f.blockA, true, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d, durableStoreFilenameV065)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := gateReadV065(t, d, b, f)
	r.Snapshot.HasLock = true
	r.Snapshot.LockedBlock = f.blockB
	forged, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, forged, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := gatePrevoteV065(d, f, f.blockA, true, nil); err == nil {
		t.Fatal("accepted forged lock snapshot")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	r = gateReadV065(t, d, b, f)
	r.Events[0].Proposal = f.blockB
	forged, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, forged, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := gatePrecommitV065(d, f, nil, false); err == nil {
		t.Fatal("accepted event history mismatching vote")
	}
}

func TestGateV065FailureBeforeAndAfterRename(t *testing.T) {
	f, d, b := gateFixtureV065(t)
	e := durableGateEventV065{Kind: durableGatePrevoteV065, Proposal: f.blockA, BlockValidated: true}
	hook := func(stage string) error {
		if stage == "before-rename" {
			return errors.New("injected")
		}
		return nil
	}
	vote, err := durableGateApplyHookV065(d, b, f.validators, f.keys[0], e, hook)
	if err == nil || vote.Signature != "" {
		t.Fatal("pre-persist failure exposed signature")
	}
	if r := gateReadV065(t, d, b, f); r.Revision != 1 || len(r.Votes) != 0 {
		t.Fatal("failed pre-rename left votes")
	}
	hook = func(stage string) error {
		if stage == "after-rename" {
			return errors.New("injected")
		}
		return nil
	}
	vote, err = durableGateApplyHookV065(d, b, f.validators, f.keys[0], e, hook)
	if err == nil || vote.Signature != "" {
		t.Fatal("post-rename error exposed signature")
	}
	r := gateReadV065(t, d, b, f)
	if len(r.Votes) != 1 || r.Revision != 2 {
		t.Fatal("renamed state not recoverable")
	}
	retry, err := gatePrevoteV065(d, f, f.blockA, true, nil)
	if err != nil || retry != r.Votes[0] {
		t.Fatal("retry did not recover original signed vote", err)
	}
	if gateReadV065(t, d, b, f).Revision != 2 {
		t.Fatal("retry changed revision")
	}
}

func TestGateV065RejectWrongSnapshotSignerAndInvalidPoLC(t *testing.T) {
	f, d, b := gateFixtureV065(t)
	fake := append([]consensus.Validator(nil), f.validators...)
	fake[0].Stake++
	if _, err := SignDurableGatePrevoteV065(d, "prism-lock-lab", f.genesis, 3, fake, f.keys[0], f.blockA, true, nil); err == nil {
		t.Fatal("wrong snapshot accepted")
	}
	if _, err := SignDurableGatePrevoteV065(d, "prism-lock-lab", f.genesis, 3, f.validators, f.keys[2], f.blockA, true, nil); err == nil {
		t.Fatal("wrong key used for durable binding")
	}
	if _, err := gatePrevoteV065(d, f, f.blockA, true, nil); err != nil {
		t.Fatal(err)
	}
	cert := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	if _, err := gatePrecommitV065(d, f, &cert, true); err != nil {
		t.Fatal(err)
	}
	if err := gateAdvanceV065(d, f, 1); err != nil {
		t.Fatal(err)
	}
	stale := f.certificate(t, stepPrevote, f.blockB, 0, 0, 1)
	if _, err := gatePrevoteV065(d, f, f.blockB, true, &stale); err == nil {
		t.Fatal("stale PoLC bypassed lock")
	}
	r := gateReadV065(t, d, b, f)
	if !r.Snapshot.HasLock || r.Snapshot.Step != BFTProposeV065 {
		t.Fatal("failed PoLC mutated state")
	}
}

func TestGateV065ChildProcess(t *testing.T) {
	dir := os.Getenv("PRISM_GATE_CHILD_DIR")
	if dir == "" {
		return
	}
	f := fixtureLockV065(t)
	proposal := os.Getenv("PRISM_GATE_CHILD_PROPOSAL")
	if _, err := gatePrevoteV065(dir, f, proposal, true, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGateV065MultiProcessConflictingProposals(t *testing.T) {
	f, d, b := gateFixtureV065(t)
	const n = 10
	var wg sync.WaitGroup
	var ok, fail int
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		proposal := f.blockA
		if i%2 == 1 {
			proposal = f.blockB
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestGateV065ChildProcess$")
			cmd.Env = append(os.Environ(), "PRISM_GATE_CHILD_DIR="+d, "PRISM_GATE_CHILD_PROPOSAL="+proposal)
			_, err := cmd.CombinedOutput()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fail++
			} else {
				ok++
			}
		}()
	}
	wg.Wait()
	if ok != n/2 || fail != n/2 {
		t.Fatalf("concurrent results ok=%d fail=%d", ok, fail)
	}
	r := gateReadV065(t, d, b, f)
	if len(r.Votes) != 1 || len(r.Events) != 1 || r.Revision != 2 {
		t.Fatalf("unexpected record: votes=%d events=%d revision=%d", len(r.Votes), len(r.Events), r.Revision)
	}
	if r.Votes[0].Context.BlockHash != f.blockA && r.Votes[0].Context.BlockHash != f.blockB {
		t.Fatal("unknown winning hash")
	}
}

func TestGateV065CommitObservationAndImmutability(t *testing.T) {
	f, d, b := gateFixtureV065(t)
	if _, err := gatePrevoteV065(d, f, f.blockA, true, nil); err != nil {
		t.Fatal(err)
	}
	qa := f.certificate(t, stepPrevote, f.blockA, 0, 0, 1)
	if _, err := gatePrecommitV065(d, f, &qa, true); err != nil {
		t.Fatal(err)
	}
	cp := f.certificate(t, stepPrecommit, f.blockA, 0, 0, 1)
	if err := ObserveDurableGateCommitV065(d, "prism-lock-lab", f.genesis, 3, f.validators, f.keys[0], cp, true); err != nil {
		t.Fatal(err)
	}
	r := gateReadV065(t, d, b, f)
	if !r.Snapshot.Finalized || r.Snapshot.FinalizedBlock != f.blockA {
		t.Fatal("failed persisted commit observation")
	}
	if _, err := gatePrevoteV065(d, f, f.blockB, true, nil); err == nil {
		t.Fatal("vote after observed commit")
	}
	if !reflect.DeepEqual(r, gateReadV065(t, d, b, f)) {
		t.Fatal("rejection changed state")
	}
}
