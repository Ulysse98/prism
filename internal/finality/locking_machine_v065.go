package finality

// Simplified deterministic, in-memory BFT transition laboratory.
// No network signing, persisted locks, NIL certificates, or live consensus.

import (
	"fmt"

	"prism/internal/consensus"
)

const (
	BFTProposeV065   = "PROPOSE"
	BFTPrevoteV065   = "PREVOTE"
	BFTPrecommitV065 = "PRECOMMIT"
	BFTCommitV065    = "COMMIT"
)

// BFTLockSnapshotV065 intentionally distinguishes no lock from a lock at round 0.
// A blank vote choice means NIL; it is NOT a signed vote in the 2C.5B lab.
type BFTLockSnapshotV065 struct {
	Height          uint64
	Round           uint64
	Step            string
	HasLock         bool
	LockedBlock     string
	LockedRound     uint64
	HasValid        bool
	ValidBlock      string
	ValidRound      uint64
	PrevoteChoice   string
	PrecommitChoice string
	Finalized       bool
	FinalizedBlock  string
	FinalizedRound  uint64
}

// BFTLockMachineV065 evaluates transitions for a single height against a
// caller-authenticated validator snapshot. It never signs or persists anything.
type BFTLockMachineV065 struct {
	chainID     string
	genesisHash string
	setHash     string
	validators  []consensus.Validator
	state       BFTLockSnapshotV065
}

func NewBFTLockMachineV065(chainID, genesisHash string, height uint64, validators []consensus.Validator) (*BFTLockMachineV065, error) {
	setHash, err := ValidatorSetHash(validators)
	if err != nil {
		return nil, err
	}
	if err := validateContext(VoteContext{
		ChainID: chainID, GenesisHash: genesisHash, Height: height,
		BlockHash: genesisHash, ValidatorSetHash: setHash, Step: stepPrevote,
	}); err != nil {
		return nil, err
	}
	return &BFTLockMachineV065{
		chainID: chainID, genesisHash: genesisHash, setHash: setHash,
		validators: append([]consensus.Validator(nil), validators...),
		state:      BFTLockSnapshotV065{Height: height, Step: BFTProposeV065},
	}, nil
}

func (m *BFTLockMachineV065) SnapshotV065() BFTLockSnapshotV065 {
	return m.state
}

// verifyEvidence checks every signature and voting weight, not merely claimed
// block/round fields. Only non-NIL certificate contexts are supported here.
func (m *BFTLockMachineV065) verifyEvidence(cert Certificate, step string) error {
	c := cert.Context
	if c.ChainID != m.chainID || c.GenesisHash != m.genesisHash ||
		c.Height != m.state.Height || c.ValidatorSetHash != m.setHash || c.Step != step {
		return fmt.Errorf("certificate context does not match local height, network, step or validator set")
	}
	if _, _, err := VerifyQuorum(m.validators, cert); err != nil {
		return fmt.Errorf("invalid signed quorum: %w", err)
	}
	return nil
}

// ChoosePrevoteV065 models the propose -> prevote transition. The caller must
// validate the proposal block itself; validBlock is a stand-in, NOT validation.
// A verified prior-round PoLC can release a different prior lock only when
// lockedRound < PoLC.round < currentRound and the PoLC proves this proposal.
// With no adequate PoLC, an existing lock is prevoted even if proposal differs.
func (m *BFTLockMachineV065) ChoosePrevoteV065(proposal string, validBlock bool, pol *Certificate) (string, error) {
	if m.state.Step != BFTProposeV065 || m.state.Finalized {
		return "", fmt.Errorf("PREVOTE can only be chosen from PROPOSE")
	}
	if proposal != "" && !validHash(proposal) {
		return "", fmt.Errorf("noncanonical proposed block hash")
	}
	if proposal == "" && validBlock {
		return "", fmt.Errorf("a missing proposal cannot be marked valid")
	}
	if pol != nil {
		if proposal == "" || !validBlock {
			return "", fmt.Errorf("cannot justify absent or invalid proposal")
		}
		if err := m.verifyEvidence(*pol, stepPrevote); err != nil {
			return "", err
		}
		if pol.Context.BlockHash != proposal || pol.Context.Round >= m.state.Round {
			return "", fmt.Errorf("PoLC must prove proposed block in an earlier round")
		}
		if m.state.HasLock && pol.Context.Round <= m.state.LockedRound &&
			proposal != m.state.LockedBlock {
			return "", fmt.Errorf("stale PoLC cannot release a newer lock")
		}
	}

	// All checks above are non-mutating. Now apply the chosen transition.
	if m.state.HasLock && proposal != m.state.LockedBlock &&
		pol != nil && pol.Context.Round > m.state.LockedRound {
		m.state.HasLock = false
		m.state.LockedBlock = ""
		m.state.LockedRound = 0
	}
	choice := "" // Explicitly the NIL choice for this isolated state machine.
	if m.state.HasLock {
		choice = m.state.LockedBlock
	} else if validBlock && proposal != "" {
		choice = proposal
	}
	m.state.PrevoteChoice = choice
	m.state.Step = BFTPrevoteV065
	return choice, nil
}

// EnterPrecommitV065 models a verified non-NIL PREVOTE quorum at the current
// round for a locally validated block. Without one, it recommends
// PRECOMMIT(NIL) and leaves the lock intact.
// NIL prevote quorum unlocking is intentionally NOT implemented in 2C.5E.
func (m *BFTLockMachineV065) EnterPrecommitV065(prevoteQuorum *Certificate, blockValidated bool) (string, error) {
	if m.state.Step != BFTPrevoteV065 || m.state.Finalized {
		return "", fmt.Errorf("PRECOMMIT requires PREVOTE step")
	}
	if prevoteQuorum != nil {
		if !blockValidated {
			return "", fmt.Errorf("cannot lock unavailable or unvalidated block")
		}
		if err := m.verifyEvidence(*prevoteQuorum, stepPrevote); err != nil {
			return "", err
		}
		if prevoteQuorum.Context.Round != m.state.Round {
			return "", fmt.Errorf("prevote quorum is from another round")
		}
	}
	choice := ""
	if prevoteQuorum != nil {
		choice = prevoteQuorum.Context.BlockHash
		m.state.HasLock = true
		m.state.LockedBlock = choice
		m.state.LockedRound = m.state.Round
		m.state.HasValid = true
		m.state.ValidBlock = choice
		m.state.ValidRound = m.state.Round
	}
	m.state.PrecommitChoice = choice
	m.state.Step = BFTPrecommitV065
	return choice, nil
}

// AdvanceRoundV065 simulates a timeout or higher-round transition; it does NOT
// verify timeout messages or establish liveness. No actual signatures are made.
func (m *BFTLockMachineV065) AdvanceRoundV065(next uint64) error {
	if m.state.Step != BFTPrecommitV065 || m.state.Finalized {
		return fmt.Errorf("round advance requires PRECOMMIT step")
	}
	if next <= m.state.Round {
		return fmt.Errorf("round must strictly increase")
	}
	m.state.Round = next
	m.state.Step = BFTProposeV065
	m.state.PrevoteChoice = ""
	m.state.PrecommitChoice = ""
	return nil
}

// ObserveCommitV065 verifies a non-NIL PRECOMMIT quorum and a caller-supplied
// indication that the block has been fully validated. This is a lab observation
// only, NOT distributed finality or an active blockchain commit.
func (m *BFTLockMachineV065) ObserveCommitV065(cert Certificate, blockValidated bool) error {
	if !blockValidated {
		return fmt.Errorf("cannot commit unvalidated or unavailable block")
	}
	if err := m.verifyEvidence(cert, stepPrecommit); err != nil {
		return err
	}
	if cert.Context.Round > m.state.Round {
		return fmt.Errorf("cannot observe future-round commit certificate")
	}
	if m.state.Finalized {
		if m.state.FinalizedBlock == cert.Context.BlockHash &&
			m.state.FinalizedRound == cert.Context.Round {
			return nil // Identical fully verified retry.
		}
		return fmt.Errorf("conflicting commitment at already committed height")
	}
	m.state.Finalized = true
	m.state.FinalizedBlock = cert.Context.BlockHash
	m.state.FinalizedRound = cert.Context.Round
	m.state.Step = BFTCommitV065
	return nil
}
