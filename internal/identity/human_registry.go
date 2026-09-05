package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// HumanRegistry stores Prism addresses that successfully
// completed humanity verification.
type HumanRegistry struct {
	mu                sync.Mutex
	path              string
	verifiedAddresses map[string]bool
}

func NewHumanRegistry(path string) (*HumanRegistry, error) {
	registry := &HumanRegistry{
		path:              path,
		verifiedAddresses: make(map[string]bool),
	}

	if err := registry.load(); err != nil {
		return nil, err
	}

	return registry, nil
}

func (r *HumanRegistry) MarkVerified(address string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if address == "" {
		return errors.New("human registry address cannot be empty")
	}

	if r.verifiedAddresses[address] {
		return nil
	}

	r.verifiedAddresses[address] = true

	if err := r.save(); err != nil {
		delete(r.verifiedAddresses, address)

		return fmt.Errorf(
			"could not persist verified human address: %w",
			err,
		)
	}

	return nil
}

func (r *HumanRegistry) IsVerified(address string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.verifiedAddresses[address]
}

func (r *HumanRegistry) load() error {
	data, err := os.ReadFile(r.path)

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf(
			"could not read human registry: %w",
			err,
		)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(
		data,
		&r.verifiedAddresses,
	); err != nil {
		return fmt.Errorf(
			"could not decode human registry: %w",
			err,
		)
	}

	return nil
}

func (r *HumanRegistry) save() error {
	dir := filepath.Dir(r.path)

	if err := os.MkdirAll(
		dir,
		0755,
	); err != nil {
		return err
	}

	data, err := json.MarshalIndent(
		r.verifiedAddresses,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	if err := os.WriteFile(
		r.path,
		data,
		0644,
	); err != nil {
		return err
	}

	return nil
}
