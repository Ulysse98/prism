//go:build linux

package computecloud

// Offline Linux-only advisory GPU ownership lock. This protects cooperating
// Prism Cloud processes; it is not a GPU hardware lock or isolation sandbox.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

var ErrGlobalGPUBusyV1 = errors.New("GPU global lock held by another cooperating process")

const globalGPULockFileV1 = "gpu-owner.lock"

// GlobalGPULockV1 owns one Linux flock until Release or process termination.
// Hold it for the COMPLETE worker lifetime, including subprocess cleanup.
// An orphaned GPU worker after supervisor death is NOT protected by this lock.
// Never delete a lock file to recover a stale lock: flock is kernel-managed.
type GlobalGPULockV1 struct {
	file       *os.File
	once       sync.Once
	releaseErr error
}

// TryAcquireGlobalGPULockV1 atomically requests an exclusive non-blocking
// advisory Linux flock on a regular file in a user-private directory.
// The directory parent must already exist and be trusted (normally $HOME).
func TryAcquireGlobalGPULockV1(lockDir string) (*GlobalGPULockV1, error) {
	if !filepath.IsAbs(lockDir) || filepath.Clean(lockDir) != lockDir {
		return nil, errors.New("GPU lock directory must be a clean absolute path")
	}
	if err := os.Mkdir(lockDir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create private GPU lock directory: %w", err)
	}
	dirFD, err := syscall.Open(lockDir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("GPU lock directory must not be a symlink: %w", err)
	}
	defer syscall.Close(dirFD)
	var dirStat syscall.Stat_t
	if err := syscall.Fstat(dirFD, &dirStat); err != nil {
		return nil, fmt.Errorf("stat GPU lock directory: %w", err)
	}
	if dirStat.Mode&syscall.S_IFMT != syscall.S_IFDIR ||
		dirStat.Uid != uint32(os.Geteuid()) || dirStat.Mode&077 != 0 {
		return nil, errors.New("GPU lock directory must be owned by current user and private (0700)")
	}

	fd, err := syscall.Openat(dirFD, globalGPULockFileV1,
		syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, fmt.Errorf("open GPU lock file safely: %w", err)
	}
	file := os.NewFile(uintptr(fd), filepath.Join(lockDir, globalGPULockFileV1))
	ok := false
	defer func() {
		if !ok {
			_ = file.Close()
		}
	}()

	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return nil, fmt.Errorf("stat GPU lock file: %w", err)
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Nlink != 1 ||
		st.Uid != uint32(os.Geteuid()) || st.Mode&077 != 0 {
		return nil, errors.New("GPU lock file must be a private regular file without hardlinks")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrGlobalGPUBusyV1
		}
		return nil, fmt.Errorf("flock GPU device reservation: %w", err)
	}
	// A lock on an unlinked/replaced inode offers no protection. Compare the
	// pathname and open descriptor after acquiring flock (private dir only).
	pathInfo, pathErr := os.Lstat(filepath.Join(lockDir, globalGPULockFileV1))
	var pathStat *syscall.Stat_t
	if pathErr == nil && pathInfo.Mode().IsRegular() {
		pathStat, _ = pathInfo.Sys().(*syscall.Stat_t)
	}
	if pathStat == nil || pathStat.Dev != st.Dev || pathStat.Ino != st.Ino || pathStat.Nlink != 1 {
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		return nil, errors.New("GPU lock file changed during acquisition")
	}
	ok = true
	return &GlobalGPULockV1{file: file}, nil
}

// Release drops ownership exactly once. It is safe to call multiple times.
func (l *GlobalGPULockV1) Release() error {
	if l == nil || l.file == nil {
		return errors.New("missing GPU lock owner")
	}
	l.once.Do(func() {
		errUnlock := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		errClose := l.file.Close()
		l.releaseErr = errors.Join(errUnlock, errClose)
	})
	return l.releaseErr
}
