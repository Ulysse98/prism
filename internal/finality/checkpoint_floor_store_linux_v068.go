//go:build linux

package finality

// Experimental local checkpoint store with an explicit pending external-witness
// transition. It does NOT implement a rollback-resistant trust root.
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

const (
	floorStoreVersionV068 = 1
	floorStoreFileV068    = "checkpoint-floor-v068.json"
	floorStoreLockV068    = ".checkpoint-floor-v068.lock"
	floorStoreMaxV068     = 64 << 10
)

type checkpointFloorRecordV068 struct {
	Version        int                                `json:"version"`
	Current        TrustedCheckpointFloorV068         `json:"current"`
	Witness        TrustedCheckpointFloorWitnessV068  `json:"witness"`
	Pending        *TrustedCheckpointFloorV068        `json:"pending,omitempty"`
	PendingWitness *TrustedCheckpointFloorWitnessV068 `json:"pending_witness,omitempty"`
}

func checkCheckpointWitnessV068(f TrustedCheckpointFloorV068, w TrustedCheckpointFloorWitnessV068) error {
	digest, err := CheckpointFloorDigestV068(f)
	if err != nil {
		return err
	}
	if w.Revision != f.Revision || w.Digest != digest {
		return errors.New("checkpoint witness mismatch")
	}
	return nil
}

func validateFloorRecordV068(r checkpointFloorRecordV068) error {
	if r.Version != floorStoreVersionV068 {
		return errors.New("unsupported floor-store version")
	}
	if err := checkCheckpointWitnessV068(r.Current, r.Witness); err != nil {
		return err
	}
	if (r.Pending == nil) != (r.PendingWitness == nil) {
		return errors.New("incomplete prepared checkpoint")
	}
	if r.Pending != nil {
		next := *r.Pending
		if r.Current.Revision == ^uint64(0) || next.Revision != r.Current.Revision+1 ||
			next.Anchor.ParentHeight <= r.Current.Anchor.ParentHeight ||
			next.Anchor.ChainID != r.Current.Anchor.ChainID ||
			next.Anchor.GenesisHash != r.Current.Anchor.GenesisHash {
			return errors.New("nonmonotonic prepared checkpoint")
		}
		if err := checkCheckpointWitnessV068(next, *r.PendingWitness); err != nil {
			return err
		}
	}
	return nil
}

func withFloorStoreLockV068(dir string, fn func(string) error) error {
	if dir == "" {
		return errors.New("missing floor-store directory")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("floor-store directory must be a real private 0700 directory")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return errors.New("floor-store directory owner mismatch")
	}
	lockpath := filepath.Join(dir, floorStoreLockV068)
	fd, err := syscall.Open(lockpath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), lockpath)
	defer f.Close()
	li, err := f.Stat()
	if err != nil || !li.Mode().IsRegular() || li.Mode().Perm() != 0600 {
		return errors.New("unsafe floor-store lock file")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return fn(filepath.Join(dir, floorStoreFileV068))
}

func readFloorRecordV068(path string) (checkpointFloorRecordV068, error) {
	var r checkpointFloorRecordV068
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return r, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm() != 0600 || fi.Size() < 1 || fi.Size() > floorStoreMaxV068 {
		return r, errors.New("unsafe checkpoint-floor state file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, floorStoreMaxV068+1))
	if err != nil {
		return r, err
	}
	if len(raw) == 0 || len(raw) > floorStoreMaxV068 {
		return r, errors.New("floor-store length invalid")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("malformed checkpoint store: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return r, errors.New("trailing checkpoint store data")
	}
	return r, validateFloorRecordV068(r)
}

// A successful rename followed by a failed directory fsync gives an ambiguous
// disk result. Recovery MUST reload the record and compare an independent witness.
func writeFloorRecordV068(dir, path string, r checkpointFloorRecordV068, hook func(string) error) error {
	if err := validateFloorRecordV068(r); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(raw) > floorStoreMaxV068 {
		return errors.New("floor-store record too large")
	}
	tmp, err := os.CreateTemp(dir, ".floor-v068-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if hook != nil {
		if err = hook("before-rename"); err != nil {
			return err
		}
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if hook != nil {
		if err = hook("after-rename"); err != nil {
			return err
		}
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	closeErr := d.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// InitCheckpointFloorStoreV068 requires a PRE-CREATED fresh private directory.
// Its witness must come from an independent authenticated source; the store
// itself cannot prove that independent provenance or prevent later re-init.
func InitCheckpointFloorStoreV068(dir string, floor TrustedCheckpointFloorV068, witness TrustedCheckpointFloorWitnessV068) error {
	if err := checkCheckpointWitnessV068(floor, witness); err != nil {
		return err
	}
	return withFloorStoreLockV068(dir, func(path string) error {
		_, err := os.Lstat(path)
		if err == nil {
			return errors.New("checkpoint floor already initialized")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return writeFloorRecordV068(dir, path, checkpointFloorRecordV068{
			Version: floorStoreVersionV068, Current: floor, Witness: witness,
		}, nil)
	})
}

// ReadCheckpointFloorStoreV068 is fail-closed with pending or missing state.
// The supplied witness must be independently authenticated and monotonically
// retained; a caller-managed old witness permits rollback.
func ReadCheckpointFloorStoreV068(dir string, external TrustedCheckpointFloorWitnessV068) (TrustedCheckpointFloorV068, error) {
	var out TrustedCheckpointFloorV068
	err := withFloorStoreLockV068(dir, func(path string) error {
		r, err := readFloorRecordV068(path)
		if err != nil {
			return err
		}
		if r.Pending != nil {
			return errors.New("pending external witness update: explicit recovery required")
		}
		if r.Witness != external {
			return errors.New("external checkpoint witness does not match local state")
		}
		out = r.Current
		return nil
	})
	return out, err
}

// PrepareCheckpointFloorAdvanceV068 verifies the exact signed candidate via
// 1B.3A while holding an exclusive cooperating-process lock, then persists a
// pending transition. No new witness is published. Caller must update an
// INDEPENDENT witness service before calling CommitPreparedCheckpointFloorV068.
func PrepareCheckpointFloorAdvanceV068(
	dir string, parent *blockchain.Blockchain, validators []consensus.Validator,
	signed SignedValidatorCheckpointV068, policy CheckpointTrustPolicyV068,
	external TrustedCheckpointFloorWitnessV068,
) (TrustedCheckpointFloorWitnessV068, error) {
	return prepareCheckpointFloorHookV068(dir, parent, validators, signed, policy, external, nil)
}

func prepareCheckpointFloorHookV068(
	dir string, parent *blockchain.Blockchain, validators []consensus.Validator,
	signed SignedValidatorCheckpointV068, policy CheckpointTrustPolicyV068,
	external TrustedCheckpointFloorWitnessV068, hook func(string) error,
) (TrustedCheckpointFloorWitnessV068, error) {
	var pending TrustedCheckpointFloorWitnessV068
	err := withFloorStoreLockV068(dir, func(path string) error {
		r, err := readFloorRecordV068(path)
		if err != nil {
			return err
		}
		if r.Pending != nil {
			return errors.New("existing pending transition: reconcile before new prepare")
		}
		if r.Witness != external {
			return errors.New("external witness differs from stored floor")
		}
		next, nextWitness, err := VerifyAndAdvanceCheckpointFloorV068(parent, validators, signed, policy, r.Current, external)
		if err != nil {
			return err
		}
		if next == r.Current {
			pending = external
			return nil
		}
		r.Pending, r.PendingWitness = &next, &nextWitness
		if err := writeFloorRecordV068(dir, path, r, hook); err != nil {
			return err
		}
		pending = nextWitness
		return nil
	})
	return pending, err
}

// CommitPreparedCheckpointFloorV068 requires independently verified receipt of
// the new external witness. A caller that invents the witness has NO security.
// No implicit commit or automatic recovery is performed if witness is stale.
func CommitPreparedCheckpointFloorV068(dir string, externalNext TrustedCheckpointFloorWitnessV068) error {
	return withFloorStoreLockV068(dir, func(path string) error {
		r, err := readFloorRecordV068(path)
		if err != nil {
			return err
		}
		if r.Pending == nil || r.PendingWitness == nil {
			return errors.New("no prepared checkpoint floor")
		}
		if *r.PendingWitness != externalNext {
			return errors.New("external next witness mismatches pending floor")
		}
		r.Current, r.Witness = *r.Pending, *r.PendingWitness
		r.Pending, r.PendingWitness = nil, nil
		return writeFloorRecordV068(dir, path, r, nil)
	})
}
