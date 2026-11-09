//go:build linux

package governancev069

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrAdvisoryVoteAlreadyReserved = errors.New("advisory vote already reserved for voter and proposal")

// AdvisoryVoteReplayStore is a Linux-only, *single-host local filesystem* lab.
// It deliberately does not claim global agreement, quorum, chain finality,
// permission/eligibility, or cross-host atomicity.
// The caller must keep the directory on a trusted, local Linux filesystem.
type AdvisoryVoteReplayStore struct{ dir string }

// OpenAdvisoryVoteReplayStore creates/opens a private (0700) owned directory.
// Parent directories must already exist and be trusted. Refuses symlink path
// components. No RPC, network calls, validator signing, or consensus writes.
func OpenAdvisoryVoteReplayStore(dir string) (*AdvisoryVoteReplayStore, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || dir == "/" {
		return nil, errors.New("store directory must be a normalized absolute path")
	}
	if err := rejectSymlinkComponents(dir); err != nil {
		return nil, err
	}
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("vote replay directory must be a real private 0700 directory")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("vote replay directory is not owned by current user")
	}
	return &AdvisoryVoteReplayStore{dir: dir}, nil
}

func rejectSymlinkComponents(path string) error {
	current := "/"
	rest := path[1:]
	for len(rest) > 0 {
		idx := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				idx = i
				break
			}
		}
		component := rest
		if idx >= 0 {
			component = rest[:idx]
			rest = rest[idx+1:]
		} else {
			rest = ""
		}
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("check path component %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path component disallowed: %s", current)
		}
	}
	return nil
}

// ReserveVerifiedAdvisoryVote verifies the signature and independently
// supplied proposal/context BEFORE attempting a no-overwrite reservation.
// O_EXCL ensures one winner per key on a trusted local Linux filesystem, even
// under concurrent processes. A crash may leave an incomplete reservation:
// it then blocks future votes (fail-closed). Do not delete it automatically.
func (s *AdvisoryVoteReplayStore) ReserveVerifiedAdvisoryVote(v SignedAdvisoryVote, p ProposalIntent, trusted Context) ([32]byte, error) {
	if s == nil {
		return [32]byte{}, errors.New("nil replay store")
	}
	if err := VerifyAdvisoryVote(v, p, trusted); err != nil {
		return [32]byte{}, fmt.Errorf("vote verification rejected: %w", err)
	}
	// Phase 1B.6: the writer and closer use the SAME directory flock.
	// Even a partial/invalid closure marker rejects all new admissions.
	gate, err := localAdvisoryGate(s.dir)
	if err != nil {
		return [32]byte{}, fmt.Errorf("admission gate: %w", err)
	}
	defer gate.Close()
	closed, err := gateHasClosure(int(gate.Fd()))
	if err != nil {
		return [32]byte{}, err
	}
	if closed {
		return [32]byte{}, ErrAdvisoryBallotClosed
	}
	key, err := VoteReplayKey(v.Vote)
	if err != nil {
		return [32]byte{}, err
	}
	name := hex.EncodeToString(key[:]) + ".vote"
	// Open relative to the locked directory FD to avoid a path-switch race.
	fd, err := syscall.Openat(int(gate.Fd()), name,
		syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if errors.Is(err, syscall.EEXIST) {
		return [32]byte{}, ErrAdvisoryVoteAlreadyReserved
	}
	if err != nil {
		return [32]byte{}, fmt.Errorf("atomic local vote reservation: %w", err)
	}
	f := os.NewFile(uintptr(fd), name)
	// Once O_EXCL succeeded, never automatically delete the filename even
	// following an I/O error: this avoids re-admission after an ambiguous crash.
	digest, err := AdvisoryVoteCommitment(v.Vote)
	if err != nil {
		f.Close()
		return [32]byte{}, err
	}
	record := struct {
		Key        string             `json:"replay_key"`
		VoteSHA256 string             `json:"vote_sha256"`
		Signed     SignedAdvisoryVote `json:"signed"`
	}{hex.EncodeToString(key[:]), hex.EncodeToString(digest[:]), v}
	payload, err := json.Marshal(record)
	if err != nil {
		f.Close()
		return [32]byte{}, err
	}
	payload = append(payload, '\n')
	n, err := f.Write(payload)
	if err != nil || n != len(payload) {
		f.Close()
		return [32]byte{}, fmt.Errorf("write reservation: %d/%d bytes: %w", n, len(payload), err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return [32]byte{}, fmt.Errorf("sync reservation: %w", err)
	}
	if err = f.Close(); err != nil {
		return [32]byte{}, fmt.Errorf("close reservation: %w", err)
	}
	if err := gate.Sync(); err != nil {
		return [32]byte{}, fmt.Errorf("sync locked reservation directory: %w", err)
	}
	return key, nil
}
