package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// WorldIDProof contains only the information Prism needs
// to verify human uniqueness.
//
// Prism does not store biometric data, passports,
// or other personal information.
type WorldIDProof struct {
	Nullifier string `json:"nullifier"`
	Proof     string `json:"proof"`
	Action    string `json:"action"`
}

// WorldIDVerifier verifies World ID proofs and persists
// consumed nullifiers so they cannot be replayed across
// separate Prism CLI executions.
type WorldIDVerifier struct {
	mu             sync.Mutex
	path           string
	usedNullifiers map[string]bool
}

// NewWorldIDVerifier creates a verifier backed by a JSON file.
func NewWorldIDVerifier(path string) (*WorldIDVerifier, error) {
	v := &WorldIDVerifier{
		path:           path,
		usedNullifiers: make(map[string]bool),
	}

	if err := v.load(); err != nil {
		return nil, err
	}

	return v, nil
}

// Verify checks a World ID proof.
//
// ETHOnline prototype rules:
//
//  1. proof must exist
//  2. nullifier must exist
//  3. action must match
//  4. nullifier must never have been consumed before
func (v *WorldIDVerifier) Verify(
	proof WorldIDProof,
	expectedAction string,
) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if proof.Proof == "" {
		return errors.New("world id proof cannot be empty")
	}

	if proof.Nullifier == "" {
		return errors.New("world id nullifier cannot be empty")
	}

	if proof.Action == "" {
		return errors.New("world id action cannot be empty")
	}

	if proof.Action != expectedAction {
		return fmt.Errorf(
			"invalid world id action: expected %q, got %q",
			expectedAction,
			proof.Action,
		)
	}

	if v.usedNullifiers[proof.Nullifier] {
		return errors.New("world id nullifier already used")
	}

	v.usedNullifiers[proof.Nullifier] = true

	if err := v.save(); err != nil {
		// Roll back the in-memory state if persistence fails.
		delete(v.usedNullifiers, proof.Nullifier)

		return fmt.Errorf(
			"could not persist world id nullifier: %w",
			err,
		)
	}

	return nil
}

// IsUsed reports whether a nullifier has already been consumed.
func (v *WorldIDVerifier) IsUsed(nullifier string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.usedNullifiers[nullifier]
}

// load restores previously consumed nullifiers from disk.
func (v *WorldIDVerifier) load() error {
	data, err := os.ReadFile(v.path)

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf(
			"could not read world id state: %w",
			err,
		)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(
		data,
		&v.usedNullifiers,
	); err != nil {
		return fmt.Errorf(
			"could not decode world id state: %w",
			err,
		)
	}

	return nil
}

// save persists consumed nullifiers to disk.
//
// The file is written directly for compatibility with
// the current Windows-based Prism development environment.
func (v *WorldIDVerifier) save() error {
	dir := filepath.Dir(v.path)

	if err := os.MkdirAll(
		dir,
		0755,
	); err != nil {
		return err
	}

	data, err := json.MarshalIndent(
		v.usedNullifiers,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	if err := os.WriteFile(
		v.path,
		data,
		0644,
	); err != nil {
		return err
	}

	return nil
}
