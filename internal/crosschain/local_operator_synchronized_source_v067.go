package crosschain

import (
	"crypto/ed25519"
	"fmt"
	"sync"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// LocalSynchronizedOperatorSourceV067 is an OFFLINE, cooperative writer/read
// boundary. It owns independently copied source state. No mutable source
// pointers are returned to external callers. This does NOT make an existing
// Prism node's unsynchronized writers safe.
type LocalSynchronizedOperatorSourceV067 struct {
	mu    sync.RWMutex
	chain *blockchain.Blockchain
	pos   *consensus.ProofOfStake
}

// NewLocalSynchronizedOperatorSourceV067 takes a defensive copy of initial
// trusted state. The caller MUST quiesce/synchronize its original source and
// validators for the duration of this INITIAL capture.
func NewLocalSynchronizedOperatorSourceV067(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) (*LocalSynchronizedOperatorSourceV067, error) {
	chainCopy, posCopy, err := snapshotOperatorSourceV067(chain, pos)
	if err != nil {
		return nil, err
	}
	return &LocalSynchronizedOperatorSourceV067{chain: chainCopy, pos: posCopy}, nil
}

// Capture returns independently validated deep copies while holding the read
// lock. Only writers using ApplyTrustedUpdateV067 are synchronized by this lock.
func (s *LocalSynchronizedOperatorSourceV067) Capture() (*blockchain.Blockchain, *consensus.ProofOfStake, error) {
	if s == nil {
		return nil, nil, ErrOperatorTrustV067
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.chain == nil || s.pos == nil {
		return nil, nil, ErrOperatorTrustV067
	}
	return snapshotOperatorSourceV067(s.chain, s.pos)
}

// ApplyTrustedUpdateV067 validates a copy-on-write candidate and publishes it
// atomically. Callbacks MUST be trusted and must NOT mutate escaped pointers
// concurrently or reenter this source. This is a lab contract, not a network
// or consensus mutation API.
func (s *LocalSynchronizedOperatorSourceV067) ApplyTrustedUpdateV067(
	update func(*blockchain.Blockchain, *consensus.ProofOfStake) error,
) error {
	if s == nil || update == nil {
		return ErrOperatorTrustV067
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.chain == nil || s.pos == nil {
		return ErrOperatorTrustV067
	}
	candidateChain, candidatePos, err := snapshotOperatorSourceV067(s.chain, s.pos)
	if err != nil {
		return err
	}
	if err := update(candidateChain, candidatePos); err != nil {
		return fmt.Errorf("%w: trusted update rejected: %v", ErrOperatorTrustV067, err)
	}
	// Re-copy: even if a trusted callback retained its candidate pointers,
	// it cannot mutate the published source AFTER this method returns.
	publishedChain, publishedPos, err := snapshotOperatorSourceV067(candidateChain, candidatePos)
	if err != nil {
		return err
	}
	s.chain, s.pos = publishedChain, publishedPos
	return nil
}

// NewLocalOperatorAdmitterFromSynchronizedSourceV067 uses one validated,
// detached source view. The root key must come from trusted provisioning;
// this wrapper does NOT establish key provenance, BFT finality, or payment
// authority. Old constructors remain available and are NOT forced through it.
func NewLocalOperatorAdmitterFromSynchronizedSourceV067(
	store *LocalReplayStoreV067,
	pinnedOperatorKey ed25519.PublicKey,
	signed SignedOperatorTrustV067,
	source *LocalSynchronizedOperatorSourceV067,
) (*LocalOperatorAdmitterV067, error) {
	if source == nil {
		return nil, ErrOperatorTrustV067
	}
	chain, pos, err := source.Capture()
	if err != nil {
		return nil, err
	}
	return NewLocalOperatorAdmitterV067(store, pinnedOperatorKey, signed, chain, pos)
}
