package crosschain

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

func TestLocalSynchronizedSourceV067DetachedAndDefensiveCapture(t *testing.T) {
	f, _ := newCheckpointFixtureV067(t)
	source, err := NewLocalSynchronizedOperatorSourceV067(f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	originalGenesis := f.chain.Blocks[0].Hash
	originalStake := f.pos.Validators[0].Stake
	f.chain.Blocks[0].Hash = strings.Repeat("f", 64)
	f.pos.Validators[0].Stake++
	t.Cleanup(func() {
		f.chain.Blocks[0].Hash = originalGenesis
		f.pos.Validators[0].Stake = originalStake
	})
	chainA, posA, err := source.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if chainA.Blocks[0].Hash != originalGenesis || posA.Validators[0].Stake != originalStake {
		t.Fatal("owned source followed a caller-side mutation")
	}
	chainA.Blocks[0].Hash = strings.Repeat("e", 64)
	posA.Validators[0].Stake++
	chainB, posB, err := source.Capture()
	if err != nil {
		t.Fatal(err)
	}
	if chainB.Blocks[0].Hash != originalGenesis || posB.Validators[0].Stake != originalStake {
		t.Fatal("Capture exposed private mutable state")
	}
}

func TestLocalSynchronizedSourceV067InvalidWriteDoesNotPublish(t *testing.T) {
	f, _ := newCheckpointFixtureV067(t)
	source, err := NewLocalSynchronizedOperatorSourceV067(f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.ApplyTrustedUpdateV067(func(c *blockchain.Blockchain, _ *consensus.ProofOfStake) error {
		c.Blocks[0].Hash = strings.Repeat("f", 64)
		return nil
	}); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("expected invalid candidate to fail closed, got %v", err)
	}
	if _, _, err := source.Capture(); err != nil {
		t.Fatalf("failed write poisoned previously valid state: %v", err)
	}
	var leaked *blockchain.Blockchain
	if err := source.ApplyTrustedUpdateV067(func(c *blockchain.Blockchain, _ *consensus.ProofOfStake) error {
		leaked = c
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	leaked.Blocks[0].Hash = strings.Repeat("e", 64)
	if _, _, err := source.Capture(); err != nil {
		t.Fatalf("callback-retained pointer changed published state: %v", err)
	}
	if err := source.ApplyTrustedUpdateV067(nil); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("missing trusted writer accepted: %v", err)
	}
}

func TestLocalSynchronizedSourceV067CaptureWaitsForTrustedWriter(t *testing.T) {
	f, _ := newCheckpointFixtureV067(t)
	source, err := NewLocalSynchronizedOperatorSourceV067(f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- source.ApplyTrustedUpdateV067(func(_ *blockchain.Blockchain, _ *consensus.ProofOfStake) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	captureDone := make(chan error, 1)
	go func() {
		_, _, err := source.Capture()
		captureDone <- err
	}()
	select {
	case err := <-captureDone:
		t.Fatalf("Capture bypassed an active writer lock: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	close(release)
	released = true
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if err := <-captureDone; err != nil {
		t.Fatal(err)
	}
}

func TestLocalSynchronizedSourceV067RealAdmissionAndReplay(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	source, err := NewLocalSynchronizedOperatorSourceV067(f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedOperatorTrustV067{
		Policy:     f.candidate.policy,
		Checkpoint: cp,
		Signature:  ed25519.Sign(priv, msg),
	}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	operator, err := newLocalOperatorAdmitterFromSynchronizedSourceV067(store, pub, signed, source)
	if err != nil {
		t.Fatal(err)
	}
	candidate := LocalAnchorCandidateV067{
		Intent:   f.candidate.intent,
		Job:      f.candidate.job,
		Contract: f.candidate.contract,
		Proof:    f.candidate.proof,
		Report:   f.candidate.report,
	}
	if _, err := operator.Admit(candidate); err != nil {
		t.Fatal(err)
	}
	if _, err := operator.Admit(candidate); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("same verified candidate was accepted twice: %v", err)
	}
	if _, err := newLocalOperatorAdmitterFromSynchronizedSourceV067(store, pub, signed, nil); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("nil source accepted: %v", err)
	}
}
