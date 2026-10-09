package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const prismAPIRuntimeLockName = "prism-api-active.lock"

// prismAPIRuntimeLock represents exclusive ownership of
// one Prism API data directory.
//
// The owner must hold this lock until the HTTP server has
// stopped serving requests.
//
// This is a local filesystem lock, not a distributed lease.
// A process crash intentionally leaves a stale lock.
type prismAPIRuntimeLock struct {
	path       string
	once       sync.Once
	releaseErr error
}

func acquirePrismAPIRuntimeLock(
	dataDir string,
) (*prismAPIRuntimeLock, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf(
			"missing Prism API data directory",
		)
	}

	info, err := os.Lstat(dataDir)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to inspect Prism API data directory: %w",
			err,
		)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf(
			"Prism API data path is not a directory",
		)
	}

	path := filepath.Join(
		dataDir,
		prismAPIRuntimeLockName,
	)

	// Atomic directory creation prevents cooperating
	// processes from acquiring the same local lock.
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, fmt.Errorf(
			"Prism API data directory is locked or unavailable: %w",
			err,
		)
	}

	return &prismAPIRuntimeLock{
		path: path,
	}, nil
}

// Release must only be called after the HTTP server has
// stopped accepting requests AND finished active handlers.
func (lock *prismAPIRuntimeLock) Release() error {
	if lock == nil {
		return fmt.Errorf("nil Prism API runtime lock")
	}

	lock.once.Do(func() {
		lock.releaseErr = os.Remove(lock.path)
	})

	return lock.releaseErr
}
