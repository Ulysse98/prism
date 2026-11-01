package crosschain

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

const operatorTrustDomainV067 = "PRISM/CROSSCHAIN/LOCAL-OPERATOR-TRUST/V067\x00"

var ErrOperatorTrustV067 = errors.New("v0.67 operator trust invalid")

// SignedOperatorTrustV067 binds a single, operator-approved policy and local
// checkpoint to one offline admission context. The public signing key MUST
// come from an independent, trusted operator channel, NOT from an applicant.
// This signature is not proof of BFT finality or settlement authority.
type SignedOperatorTrustV067 struct {
	Policy     AnchorProofPolicyV067
	Checkpoint PinnedSourceCheckpointV067
	Signature  []byte
}

type operatorTrustPayloadV067 struct {
	Version    uint32                     `json:"version"`
	Policy     AnchorProofPolicyV067      `json:"policy"`
	Checkpoint PinnedSourceCheckpointV067 `json:"checkpoint"`
}

// operatorTrustMessageV067 gives one domain-separated serialization for a
// signed offline Go trust bundle. It is a local v0.67 format, not an external
// cross-chain signature standard.
func operatorTrustMessageV067(policy AnchorProofPolicyV067, cp PinnedSourceCheckpointV067) ([]byte, error) {
	if !chainIDV067.MatchString(policy.SourceChainID) ||
		!chainIDV067.MatchString(policy.DestinationChainID) ||
		!registryIDV067.MatchString(policy.DestinationRegistry) ||
		policy.ApprovedVerifier == "" ||
		cp.SourceChainID != policy.SourceChainID ||
		cp.SourceGenesisHash != policy.SourceGenesisHash ||
		cp.MinSuccessorBlocks < 1 || cp.MinSuccessorBlocks > 128 ||
		cp.CheckpointHeight == 0 {
		return nil, fmt.Errorf("%w: policy or checkpoint identity/depth", ErrOperatorTrustV067)
	}
	if err := validHashV067("policy genesis", policy.SourceGenesisHash); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperatorTrustV067, err)
	}
	if err := validHashV067("approved artifact", policy.ApprovedArtifactSHA); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperatorTrustV067, err)
	}
	if err := validHashV067("checkpoint block", cp.CheckpointBlockHash); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperatorTrustV067, err)
	}
	data, err := json.Marshal(operatorTrustPayloadV067{
		Version: 1, Policy: policy, Checkpoint: cp,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: encode bundle: %v", ErrOperatorTrustV067, err)
	}
	message := make([]byte, 0, len(operatorTrustDomainV067)+len(data))
	message = append(message, operatorTrustDomainV067...)
	message = append(message, data...)
	return message, nil
}

// LocalOperatorAdmitterV067 fixes the signer-verified policy and checkpoint
// during initialization; Admit accepts ONLY untrusted candidate material.
// The operator owns the store; caller must synchronize source inputs while
// constructing its independent snapshot. Not a network-facing trust boundary.
type LocalOperatorAdmitterV067 struct {
	store      *LocalReplayStoreV067
	policy     AnchorProofPolicyV067
	checkpoint PinnedSourceCheckpointV067
	chain      *blockchain.Blockchain
	pos        *consensus.ProofOfStake
}

// NewLocalOperatorAdmitterV067 may ONLY be called by trusted operator code.
// pinnedOperatorKey MUST be loaded from a trusted external config or binary;
// accepting a candidate-provided key defeats the signature check entirely.
func newLocalOperatorAdmitterV067(
	store *LocalReplayStoreV067,
	pinnedOperatorKey ed25519.PublicKey,
	signed SignedOperatorTrustV067,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) (*LocalOperatorAdmitterV067, error) {
	if store == nil || store.dir == "" || chain == nil || pos == nil ||
		len(pinnedOperatorKey) != ed25519.PublicKeySize ||
		len(signed.Signature) != ed25519.SignatureSize {
		return nil, fmt.Errorf("%w: missing or malformed trusted initialization", ErrOperatorTrustV067)
	}
	message, err := operatorTrustMessageV067(signed.Policy, signed.Checkpoint)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(pinnedOperatorKey, message, signed.Signature) {
		return nil, fmt.Errorf("%w: signature not from pinned operator", ErrOperatorTrustV067)
	}
	// Retain independently validated copies. Caller must synchronize source
	// access while this snapshot is being materialized.
	snapshotChain, snapshotPos, err := snapshotOperatorSourceV067(chain, pos)
	if err != nil {
		return nil, err
	}
	return &LocalOperatorAdmitterV067{
		store: store, policy: signed.Policy, checkpoint: signed.Checkpoint,
		chain: snapshotChain, pos: snapshotPos,
	}, nil
}

// Admit does not accept a request-defined operator key, policy or checkpoint.
// A successful reservation is LOCAL evidence only; it NEVER authorizes payout,
// escrow release, transaction signing, or cross-chain registration.
func (a *LocalOperatorAdmitterV067) Admit(candidate LocalAnchorCandidateV067) (LocalReplayReservationV067, error) {
	if a == nil || a.store == nil || a.chain == nil || a.pos == nil {
		return LocalReplayReservationV067{}, ErrOperatorTrustV067
	}
	return admitLocalAnchorCandidateV067(a.store, candidate, a.policy, a.checkpoint, a.chain, a.pos)
}
