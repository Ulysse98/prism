//go:build linux

package finality

// Research-only atomic storage boundary. NOT a BFT signing gate.
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"prism/internal/blockchain"
)

const (
	durableStoreVersionV065  = 1
	durableStoreFilenameV065 = "bft-safety-v065.json"
	durableStoreLocknameV065 = ".bft-safety-v065.lock"
	durableStoreMaxBytesV065 = 8 << 20
)

type durableBindingV065 struct {
	ChainID          string `json:"chain_id"`
	GenesisHash      string `json:"genesis_hash"`
	ValidatorSetHash string `json:"validator_set_hash"`
	SignerAddress    string `json:"signer_address"`
	Height           uint64 `json:"height"`
	Mode             string `json:"mode,omitempty"`
	ParentHash       string `json:"parent_hash,omitempty"`
}

type durableBFTRecordV065 struct {
	Version  int                    `json:"version"`
	Binding  durableBindingV065     `json:"binding"`
	Revision uint64                 `json:"revision"`
	Snapshot BFTLockSnapshotV065    `json:"snapshot"`
	Votes    []SignedVote           `json:"votes"`
	Events   []durableGateEventV065 `json:"events,omitempty"`
	Parent   *blockchain.Blockchain `json:"parent,omitempty"`
}

func validateDurableBindingV065(b durableBindingV065) error {
	if b.ChainID == "" || !validHash(b.GenesisHash) || !validHash(b.ValidatorSetHash) || b.SignerAddress == "" || b.Height == 0 {
		return errors.New("invalid durable binding")
	}
	if b.Mode != "" && (b.Mode != validatedPrevoteModeV065 || !validHash(b.ParentHash)) {
		return errors.New("invalid validated gate mode or parent hash")
	}
	if b.Mode == "" && b.ParentHash != "" {
		return errors.New("legacy binding cannot carry parent hash")
	}
	return nil
}

// Only structural validation; provenance of transitions is NOT proven here.
func validateDurableRecordV065(r durableBFTRecordV065, b durableBindingV065) error {
	if r.Version != durableStoreVersionV065 || r.Binding != b || r.Revision == 0 || r.Snapshot.Height != b.Height {
		return errors.New("durable state identity/version mismatch")
	}
	if b.Mode == validatedPrevoteModeV065 && r.Parent == nil {
		return errors.New("validated prevote state lacks pinned parent")
	}
	if b.Mode == "" && r.Parent != nil {
		return errors.New("unexpected parent witness in legacy state")
	}
	s := r.Snapshot
	switch s.Step {
	case BFTProposeV065, BFTPrevoteV065, BFTPrecommitV065, BFTCommitV065:
	default:
		return errors.New("invalid BFT step")
	}
	if s.HasLock {
		if !validHash(s.LockedBlock) || s.LockedRound > s.Round {
			return errors.New("invalid lock")
		}
	} else if s.LockedBlock != "" || s.LockedRound != 0 {
		return errors.New("inconsistent absent lock")
	}
	if s.HasValid {
		if !validHash(s.ValidBlock) || s.ValidRound > s.Round {
			return errors.New("invalid valid-block state")
		}
	} else if s.ValidBlock != "" || s.ValidRound != 0 {
		return errors.New("inconsistent absent valid block")
	}
	if (s.PrevoteChoice != "" && !validHash(s.PrevoteChoice)) || (s.PrecommitChoice != "" && !validHash(s.PrecommitChoice)) {
		return errors.New("invalid vote choice")
	}
	if s.Finalized {
		if s.Step != BFTCommitV065 || !validHash(s.FinalizedBlock) || s.FinalizedRound > s.Round {
			return errors.New("invalid commit")
		}
	} else if s.Step == BFTCommitV065 || s.FinalizedBlock != "" || s.FinalizedRound != 0 {
		return errors.New("inconsistent absent finalization")
	}
	seen := make(map[voteSlotV065]bool, len(r.Votes))
	for _, v := range r.Votes {
		if err := verifyStoredVoteV065(v); err != nil {
			return fmt.Errorf("invalid stored vote: %w", err)
		}
		c := v.Context
		if c.ChainID != b.ChainID || c.GenesisHash != b.GenesisHash || c.ValidatorSetHash != b.ValidatorSetHash || c.Height != b.Height || v.Address != b.SignerAddress {
			return errors.New("stored vote binding mismatch")
		}
		slot := slotForVoteV065(v)
		if seen[slot] {
			return errors.New("duplicate signing slot")
		}
		seen[slot] = true
	}
	return nil
}

func withDurableLockV065(dir string, action func() error) error {
	if dir == "" {
		return errors.New("missing directory")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	} // No implicit recreation after lost state.
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("unsafe durable directory")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return errors.New("directory owner mismatch")
	}
	path := filepath.Join(dir, durableStoreLocknameV065)
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	lock := os.NewFile(uintptr(fd), path)
	defer lock.Close()
	info, err = lock.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("unsafe lock file")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return action()
}

func loadDurableV065(path string, b durableBindingV065) (durableBFTRecordV065, error) {
	var record durableBFTRecordV065
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return record, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() <= 0 || info.Size() > durableStoreMaxBytesV065 {
		return record, errors.New("unsafe durable state file")
	}
	data, err := io.ReadAll(io.LimitReader(file, durableStoreMaxBytesV065+1))
	if err != nil {
		return record, err
	}
	if len(data) == 0 || len(data) > durableStoreMaxBytesV065 {
		return record, errors.New("durable state size invalid")
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return record, fmt.Errorf("corrupt durable state: %w", err)
	}
	return record, validateDurableRecordV065(record, b)
}

// hook is only for deterministic fault injection in laboratory tests.
func persistDurableV065(dir, path string, r durableBFTRecordV065, hook func(string) error) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > durableStoreMaxBytesV065 {
		return errors.New("durable state too large")
	}
	tmp, err := os.CreateTemp(dir, ".bft-safety-v065-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if hook != nil {
		if err := hook("before-rename"); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if hook != nil {
		if err := hook("after-rename"); err != nil {
			return err
		}
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := dirFile.Sync()
	closeErr := dirFile.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// Explicit fresh setup ONLY. Deletion plus re-init remains an out-of-scope rollback hazard.
func initDurableV065(dir string, b durableBindingV065) error {
	return initDurableWithParentV065(dir, b, nil)
}

func initDurableWithParentV065(dir string, b durableBindingV065, parent *blockchain.Blockchain) error {
	if err := validateDurableBindingV065(b); err != nil {
		return err
	}
	return withDurableLockV065(dir, func() error {
		path := filepath.Join(dir, durableStoreFilenameV065)
		_, err := os.Lstat(path)
		if err == nil {
			return errors.New("already initialized")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		r := durableBFTRecordV065{Version: 1, Binding: b, Revision: 1, Snapshot: BFTLockSnapshotV065{Height: b.Height, Step: BFTProposeV065}, Parent: parent}
		if err := validateDurableRecordV065(r, b); err != nil {
			return err
		}
		return persistDurableV065(dir, path, r, nil)
	})
}

func readDurableV065(dir string, b durableBindingV065) (durableBFTRecordV065, error) {
	var out durableBFTRecordV065
	if err := validateDurableBindingV065(b); err != nil {
		return out, err
	}
	err := withDurableLockV065(dir, func() error {
		var e error
		out, e = loadDurableV065(filepath.Join(dir, durableStoreFilenameV065), b)
		return e
	})
	return out, err
}

// Storage transaction only; callers must NOT use this to approve signatures.
// The storage layer cannot authenticate BFT transition provenance.
func transactDurableV065(dir string, b durableBindingV065, mutate func(*durableBFTRecordV065) error) (durableBFTRecordV065, error) {
	return transactDurableHookV065(dir, b, mutate, nil)
}
func transactDurableHookV065(dir string, b durableBindingV065, mutate func(*durableBFTRecordV065) error, hook func(string) error) (durableBFTRecordV065, error) {
	var output durableBFTRecordV065
	if err := validateDurableBindingV065(b); err != nil {
		return output, err
	}
	if mutate == nil {
		return output, errors.New("nil mutator")
	}
	err := withDurableLockV065(dir, func() error {
		path := filepath.Join(dir, durableStoreFilenameV065)
		prior, err := loadDurableV065(path, b)
		if err != nil {
			return err
		}
		data, err := json.Marshal(prior)
		if err != nil {
			return err
		}
		var next durableBFTRecordV065
		if err := json.Unmarshal(data, &next); err != nil {
			return err
		}
		if err := mutate(&next); err != nil {
			return err
		}
		if next.Revision != prior.Revision || next.Revision == ^uint64(0) {
			return errors.New("revision violation")
		}
		next.Revision++
		if err := validateDurableRecordV065(next, b); err != nil {
			return err
		}
		if err := persistDurableV065(dir, path, next, hook); err != nil {
			return err
		}
		output = next
		return nil
	})
	return output, err
}
