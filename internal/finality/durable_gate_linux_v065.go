//go:build linux

package finality

// Experimental transition-replaying signer gate. NOT active consensus.
// BlockValidated is a caller assertion, not an authenticated block proof.
import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

const (
	durableGatePrevoteV065   = "prevote"
	durableGatePrecommitV065 = "precommit"
	durableGateAdvanceV065   = "advance"
	durableGateCommitV065    = "observe-commit"
)

type durableGateEventV065 struct {
	Kind           string       `json:"kind"`
	Proposal       string       `json:"proposal,omitempty"`
	BlockValidated bool         `json:"block_validated,omitempty"`
	Evidence       *Certificate `json:"evidence,omitempty"`
	NextRound      uint64       `json:"next_round,omitempty"`
}

func durableGateBindingV065(chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey) (durableBindingV065, error) {
	var b durableBindingV065
	if len(key) != ed25519.PrivateKeySize {
		return b, errors.New("invalid signing key")
	}
	setHash, err := ValidatorSetHash(validators)
	if err != nil {
		return b, err
	}
	addr := wallet.AddressFromPublicKey(key.Public().(ed25519.PublicKey))
	allowed := false
	for _, v := range validators {
		if v.Address == addr && v.Stake > 0 {
			allowed = true
		}
	}
	if !allowed {
		return b, errors.New("signer absent from trusted validator snapshot")
	}
	b = durableBindingV065{ChainID: chainID, GenesisHash: genesisHash, ValidatorSetHash: setHash, SignerAddress: addr, Height: height}
	return b, validateDurableBindingV065(b)
}

// Explicit initialization. Requires a newly prepared 0700 directory.
func InitDurableGateV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey) error {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return err
	}
	return initDurableV065(dir, b)
}

func gateInitialMachineV065(b durableBindingV065, validators []consensus.Validator) (*BFTLockMachineV065, error) {
	setHash, err := ValidatorSetHash(validators)
	if err != nil || setHash != b.ValidatorSetHash {
		return nil, errors.New("trusted validator set mismatch")
	}
	return NewBFTLockMachineV065(b.ChainID, b.GenesisHash, b.Height, validators)
}

func gateEventTransitionV065(m *BFTLockMachineV065, e durableGateEventV065) (string, string, error) {
	switch e.Kind {
	case durableGatePrevoteV065:
		if e.NextRound != 0 {
			return "", "", errors.New("unexpected round field")
		}
		choice, err := m.ChoosePrevoteV065(e.Proposal, e.BlockValidated, e.Evidence)
		return choice, stepPrevote, err
	case durableGatePrecommitV065:
		if e.Proposal != "" || e.NextRound != 0 {
			return "", "", errors.New("unexpected precommit fields")
		}
		choice, err := m.EnterPrecommitV065(e.Evidence, e.BlockValidated)
		return choice, stepPrecommit, err
	case durableGateAdvanceV065:
		if e.Proposal != "" || e.Evidence != nil || e.BlockValidated {
			return "", "", errors.New("unexpected advance fields")
		}
		return "", "", m.AdvanceRoundV065(e.NextRound)
	case durableGateCommitV065:
		if e.Proposal != "" || e.NextRound != 0 || e.Evidence == nil {
			return "", "", errors.New("unexpected commit fields")
		}
		return "", "", m.ObserveCommitV065(*e.Evidence, e.BlockValidated)
	default:
		return "", "", fmt.Errorf("unknown gate event %q", e.Kind)
	}
}

func gateContextV065(b durableBindingV065, round uint64, step, choice string) VoteContext {
	return VoteContext{ChainID: b.ChainID, GenesisHash: b.GenesisHash, Height: b.Height, Round: round,
		Step: step, BlockHash: choice, Nil: choice == "", ValidatorSetHash: b.ValidatorSetHash}
}

// Replay validates transition provenance for a record using the caller-trusted
// validator set. It rejects arbitrary structurally well-formed snapshots.
func replayDurableGateV065(r durableBFTRecordV065, b durableBindingV065, validators []consensus.Validator) (*BFTLockMachineV065, error) {
	m, err := gateInitialMachineV065(b, validators)
	if err != nil {
		return nil, err
	}
	if len(r.Events) == 0 && len(r.Votes) != 0 {
		return nil, errors.New("votes without transition evidence")
	}
	cursor := 0
	for _, e := range r.Events {
		currentRound := m.SnapshotV065().Round
		choice, step, err := gateEventTransitionV065(m, e)
		if err != nil {
			return nil, fmt.Errorf("invalid stored transition: %w", err)
		}
		if step == "" {
			continue
		}
		if cursor >= len(r.Votes) {
			return nil, errors.New("transition lacks signed vote")
		}
		vote := r.Votes[cursor]
		cursor++
		if vote.Context != gateContextV065(b, currentRound, step, choice) || vote.Address != b.SignerAddress {
			return nil, errors.New("signed vote disagrees with recorded transition")
		}
	}
	if cursor != len(r.Votes) {
		return nil, errors.New("extra votes without transitions")
	}
	if !reflect.DeepEqual(m.SnapshotV065(), r.Snapshot) {
		return nil, errors.New("BFT snapshot not derivable from trusted evidence history")
	}
	return m, nil
}

// A checked, idempotent transition. A vote is not exposed if persistence fails.
// Hook is internal test-only fault injection.
func durableGateApplyHookV065(dir string, b durableBindingV065, validators []consensus.Validator, key ed25519.PrivateKey, event durableGateEventV065, hook func(string) error) (SignedVote, error) {
	var returned SignedVote
	if len(key) != ed25519.PrivateKeySize || wallet.AddressFromPublicKey(key.Public().(ed25519.PublicKey)) != b.SignerAddress {
		return returned, errors.New("signing key mismatch")
	}
	if _, err := gateInitialMachineV065(b, validators); err != nil {
		return returned, err
	}
	// Detach caller-owned certificate slices before processing.
	encoded, err := json.Marshal(event)
	if err != nil {
		return returned, err
	}
	var detached durableGateEventV065
	if err := json.Unmarshal(encoded, &detached); err != nil {
		return returned, err
	}
	err = withDurableLockV065(dir, func() error {
		path := filepath.Join(dir, durableStoreFilenameV065)
		prior, err := loadDurableV065(path, b)
		if err != nil {
			return err
		}
		machine, err := replayDurableGateV065(prior, b, validators)
		if err != nil {
			return err
		}
		// Idempotent retry, including unchanged persisted revision.
		if len(prior.Events) > 0 && reflect.DeepEqual(prior.Events[len(prior.Events)-1], detached) {
			if detached.Kind == durableGatePrevoteV065 || detached.Kind == durableGatePrecommitV065 {
				if len(prior.Votes) == 0 {
					return errors.New("missing retry vote")
				}
				returned = prior.Votes[len(prior.Votes)-1]
			}
			return nil
		}
		round := machine.SnapshotV065().Round
		choice, step, err := gateEventTransitionV065(machine, detached)
		if err != nil {
			return err
		}
		next := prior // clone slices before mutation
		next.Events = append(append([]durableGateEventV065(nil), prior.Events...), detached)
		if step != "" {
			var vote SignedVote
			vote, err = SignVote(gateContextV065(b, round, step, choice), key)
			if err != nil {
				return err
			}
			next.Votes = append(append([]SignedVote(nil), prior.Votes...), vote)
			returned = vote // only returned if full persistence succeeds
		}
		next.Snapshot = machine.SnapshotV065()
		if next.Revision == math.MaxUint64 {
			return errors.New("revision exhausted")
		}
		next.Revision++
		if err := validateDurableRecordV065(next, b); err != nil {
			return err
		}
		if _, err := replayDurableGateV065(next, b, validators); err != nil {
			return err
		}
		if err := persistDurableV065(dir, path, next, hook); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return SignedVote{}, err
	}
	return returned, nil
}

func SignDurableGatePrevoteV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey, proposal string, blockValidated bool, pol *Certificate) (SignedVote, error) {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return SignedVote{}, err
	}
	return durableGateApplyHookV065(dir, b, validators, key, durableGateEventV065{Kind: durableGatePrevoteV065, Proposal: proposal, BlockValidated: blockValidated, Evidence: pol}, nil)
}

func SignDurableGatePrecommitV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey, quorum *Certificate, blockValidated bool) (SignedVote, error) {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return SignedVote{}, err
	}
	return durableGateApplyHookV065(dir, b, validators, key, durableGateEventV065{Kind: durableGatePrecommitV065, Evidence: quorum, BlockValidated: blockValidated}, nil)
}

func AdvanceDurableGateRoundV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey, nextRound uint64) error {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return err
	}
	_, err = durableGateApplyHookV065(dir, b, validators, key, durableGateEventV065{Kind: durableGateAdvanceV065, NextRound: nextRound}, nil)
	return err
}

func ObserveDurableGateCommitV065(dir, chainID, genesisHash string, height uint64, validators []consensus.Validator, key ed25519.PrivateKey, cert Certificate, blockValidated bool) error {
	b, err := durableGateBindingV065(chainID, genesisHash, height, validators, key)
	if err != nil {
		return err
	}
	_, err = durableGateApplyHookV065(dir, b, validators, key, durableGateEventV065{Kind: durableGateCommitV065, Evidence: &cert, BlockValidated: blockValidated}, nil)
	return err
}

// Future: eliminate caller-supplied blockValidated bool in favor of complete
// authenticated block validation; isolate signing key from raw SignVote API.
