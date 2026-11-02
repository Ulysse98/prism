//go:build linux

package finality

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// This test-only subprocess uses no production validator or checkpoint keys.
// The 'external' witness is a test-controlled input, not a real authenticated service.
func TestV068RecoveryMultiProcessHelper(t *testing.T) {
	op := os.Getenv("PRISM_V068_CHILD_OP")
	if op == "" {
		return
	}
	dir := os.Getenv("PRISM_V068_CHILD_DIR")
	if dir == "" {
		t.Fatal("missing test child directory")
	}
	var witness TrustedCheckpointFloorWitnessV068
	if err := json.Unmarshal([]byte(os.Getenv("PRISM_V068_CHILD_WITNESS")), &witness); err != nil {
		t.Fatal(err)
	}
	var err error
	switch op {
	case "commit":
		err = CommitPreparedCheckpointFloorV068(dir, witness)
	case "read":
		_, err = ReadCheckpointFloorStoreV068(dir, witness)
	case "inspect":
		var status CheckpointFloorRecoveryStatusV068
		status, err = InspectCheckpointFloorRecoveryV068(dir, witness)
		if err == nil {
			fmt.Println("LAB_STATUS=" + string(status))
		}
	default:
		t.Fatal("unknown child operation")
	}
	if err != nil {
		t.Fatalf("LAB_REJECTED: %v", err)
	}
}

func v068RecoveryChildCommand(t *testing.T, op, dir string, witness TrustedCheckpointFloorWitnessV068) *exec.Cmd {
	t.Helper()
	raw, err := json.Marshal(witness)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestV068RecoveryMultiProcessHelper$", "-test.v")
	cmd.Env = append(os.Environ(),
		"PRISM_V068_CHILD_OP="+op,
		"PRISM_V068_CHILD_DIR="+dir,
		"PRISM_V068_CHILD_WITNESS="+string(raw),
	)
	return cmd
}

func TestV068RecoveryMultiProcessOneCommitWinner(t *testing.T) {
	bc, validators, nextSigned, policy, floor, old := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
		t.Fatal(err)
	}
	next, err := PrepareCheckpointFloorAdvanceV068(dir, bc, validators, nextSigned, policy, old)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 6
	var wg sync.WaitGroup
	results := make(chan struct {
		output string
		err    error
	}, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, e := v068RecoveryChildCommand(t, "commit", dir, next).CombinedOutput()
			results <- struct {
				output string
				err    error
			}{string(out), e}
		}()
	}
	wg.Wait()
	close(results)
	winners, expectedRejects := 0, 0
	for result := range results {
		if result.err == nil {
			winners++
			continue
		}
		if !strings.Contains(result.output, "no prepared checkpoint floor") {
			t.Fatalf("unexpected competing-process failure: %s (%v)", result.output, result.err)
		}
		expectedRejects++
	}
	if winners != 1 || expectedRejects != workers-1 {
		t.Fatalf("expected one commit winner and five rejects: winners=%d rejects=%d", winners, expectedRejects)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, next); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, old); err == nil {
		t.Fatal("stale witness accepted")
	}
}

func TestV068RecoveryMultiProcessFlockBlocksOtherProcess(t *testing.T) {
	_, _, _, _, floor, old := v068TrustFloorFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	lockerDone := make(chan error, 1)
	go func() {
		lockerDone <- withFloorStoreLockV068(dir, func(_ string) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	cmd := v068RecoveryChildCommand(t, "inspect", dir, old)
	if err := cmd.Start(); err != nil {
		close(release)
		<-lockerDone
		t.Fatal(err)
	}
	outcome := make(chan error, 1)
	go func() { outcome <- cmd.Wait() }()
	select {
	case err := <-outcome:
		close(release)
		<-lockerDone
		t.Fatalf("child completed while file lock held: %v", err)
	case <-time.After(200 * time.Millisecond):
		// The child remains blocked while another process holds the lock.
	}
	close(release)
	if err := <-lockerDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-outcome:
		if err != nil {
			t.Fatalf("child should succeed after unlock: %v", err)
		}
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		<-outcome
		t.Fatal("child did not finish after lock release")
	}
}

func TestV068RecoveryMultiProcessMissingExternalWitnessFailsClosed(t *testing.T) {
	bc, validators, nextSigned, policy, floor, old := v068ForwardCheckpointFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, old); err != nil {
		t.Fatal(err)
	}
	next, err := PrepareCheckpointFloorAdvanceV068(dir, bc, validators, nextSigned, policy, old)
	if err != nil {
		t.Fatal(err)
	}

	for _, op := range []string{"inspect", "read", "commit"} {
		output, err := v068RecoveryChildCommand(t, op, dir, TrustedCheckpointFloorWitnessV068{}).CombinedOutput()
		if err == nil || !strings.Contains(string(output), "LAB_REJECTED") {
			t.Fatalf("%s accepted unavailable external witness: %s / %v", op, output, err)
		}
	}
	state, err := InspectCheckpointFloorRecoveryV068(dir, next)
	if err != nil || state != CheckpointFloorCommitRequiredV068 {
		t.Fatalf("pending transition mutated after witness failure: %q %v", state, err)
	}
	if err := CommitPreparedCheckpointFloorV068(dir, next); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCheckpointFloorStoreV068(dir, next); err != nil {
		t.Fatal(err)
	}
}

func TestV068RecoveryMultiProcessUnauthenticatedInputNotTrustRoot(t *testing.T) {
	_, _, _, _, floor, witness := v068TrustFloorFixture(t)
	dir := v068FreshFloorDir(t)
	if err := InitCheckpointFloorStoreV068(dir, floor, witness); err != nil {
		t.Fatal(err)
	}
	out, err := v068RecoveryChildCommand(t, "inspect", dir, witness).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "LAB_STATUS=ready") {
		t.Fatalf("lab fixture read failed: %s %v", out, err)
	}
	// This is a negative assurance statement, not a claim of authentication:
	// a subprocess can supply a matching witness without proving provenance.
}
