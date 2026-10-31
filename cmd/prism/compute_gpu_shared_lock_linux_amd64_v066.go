//go:build linux && amd64

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// This is the *same advisory flock protocol* used by Prism Cloud v1.
// Both projects must choose precisely the same directory (normally
// $HOME/.prism-cloud-gpu-lock-v1), user and filesystem. Non-cooperating
// CUDA applications are unaffected. Never unlink this lock file.
var errSharedGPUBusyV066 = errors.New("GPU_BUSY: cooperative global GPU lock already held")

const sharedGPULockFileV066 = "gpu-owner.lock"

type sharedGPULockV066 struct{ file *os.File }

func tryAcquireSharedGPULockV066(lockDir string) (_ *sharedGPULockV066, err error) {
	if !filepath.IsAbs(lockDir) || filepath.Clean(lockDir) != lockDir {
		return nil, fmt.Errorf("v0.66 GPU lock directory must be a clean absolute path")
	}
	if e := os.Mkdir(lockDir, 0700); e != nil && !errors.Is(e, os.ErrExist) {
		return nil, fmt.Errorf("create GPU lock directory: %w", e)
	}
	dirFD, e := syscall.Open(lockDir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, fmt.Errorf("open private GPU lock directory: %w", e)
	}
	defer syscall.Close(dirFD)
	var ds syscall.Stat_t
	if e = syscall.Fstat(dirFD, &ds); e != nil {
		return nil, fmt.Errorf("fstat GPU lock directory: %w", e)
	}
	if ds.Mode&syscall.S_IFMT != syscall.S_IFDIR || ds.Uid != uint32(os.Geteuid()) || ds.Mode&077 != 0 {
		return nil, fmt.Errorf("GPU lock directory must be private and owned by current user")
	}
	fd, e := syscall.Openat(dirFD, sharedGPULockFileV066,
		syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if e != nil {
		return nil, fmt.Errorf("open GPU lock file safely: %w", e)
	}
	file := os.NewFile(uintptr(fd), filepath.Join(lockDir, sharedGPULockFileV066))
	defer func() {
		if err != nil {
			_ = file.Close()
		}
	}()
	var st syscall.Stat_t
	if e = syscall.Fstat(fd, &st); e != nil {
		return nil, fmt.Errorf("fstat GPU lock file: %w", e)
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Nlink != 1 || st.Uid != uint32(os.Geteuid()) || st.Mode&077 != 0 {
		return nil, fmt.Errorf("GPU lock file must be private regular file without hardlinks")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		if errors.Is(e, syscall.EWOULDBLOCK) || errors.Is(e, syscall.EAGAIN) {
			return nil, errSharedGPUBusyV066
		}
		return nil, fmt.Errorf("acquire GPU flock: %w", e)
	}
	pathInfo, pathErr := os.Lstat(filepath.Join(lockDir, sharedGPULockFileV066))
	var pst *syscall.Stat_t
	if pathErr == nil && pathInfo.Mode().IsRegular() {
		pst, _ = pathInfo.Sys().(*syscall.Stat_t)
	}
	if pst == nil || pst.Dev != st.Dev || pst.Ino != st.Ino || pst.Nlink != 1 {
		return nil, fmt.Errorf("GPU lock file changed during acquisition")
	}
	return &sharedGPULockV066{file: file}, nil
}

// The GPU subprocess inherits the lock as descriptor 4. The sealed memfd
// remains descriptor 3. Crucially, only Close is used on the parent side:
// LOCK_UN on a shared open-file description would also unlock the child.
func runSealedGPUWorkerWithSharedLockV066(
	ctx context.Context, sealed, lock *os.File, request []byte, workerArgs ...string,
) ([]byte, error) {
	return runSealedGPUWorkerWithSharedLockStartedV066(ctx, sealed, lock, request, nil, workerArgs...)
}

// started is a test-only synchronization hook signalled after Cmd.Start returns.
// This prevents closing the parent descriptor concurrently with Go's Start.
func runSealedGPUWorkerWithSharedLockStartedV066(
	ctx context.Context, sealed, lock *os.File, request []byte, started chan<- struct{}, workerArgs ...string,
) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || sealed == nil || lock == nil {
		return nil, fmt.Errorf("v0.66 missing active context, sealed binary or shared GPU lock")
	}
	seals, _, errno := syscall.Syscall(syscall.SYS_FCNTL, sealed.Fd(), fGetSealsV066, 0)
	if errno != 0 || seals&requiredSealsV066 != requiredSealsV066 {
		return nil, fmt.Errorf("v0.66 requires fully sealed worker memfd: %v", errno)
	}
	var st syscall.Stat_t
	if e := syscall.Fstat(int(lock.Fd()), &st); e != nil || st.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return nil, fmt.Errorf("v0.66 inherited GPU lock descriptor invalid: %v", e)
	}
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "/proc/self/fd/3", workerArgs...)
	cmd.ExtraFiles = []*os.File{sealed, lock}
	if err := configureGPUWorkerProcessV062(cmd); err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewReader(request)
	cmd.WaitDelay = 2 * time.Second
	stdout := &boundedGPUOutputV062{limit: 128 * 1024}
	stderr := &boundedGPUOutputV062{limit: 16 * 1024}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	startErr := cmd.Start()
	if startErr != nil {
		return nil, fmt.Errorf("v0.66 shared GPU worker start failed: %w", startErr)
	}
	if started != nil {
		close(started)
	}
	runErr := cmd.Wait()
	if runCtx.Err() != nil {
		return nil, fmt.Errorf("v0.66 locked GPU worker cancelled or timed out: %w", runCtx.Err())
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, fmt.Errorf("v0.66 locked GPU worker output exceeded limits")
	}
	if runErr != nil {
		detail := string(stderr.data)
		if len(detail) > 2000 {
			detail = detail[:2000]
		}
		return nil, fmt.Errorf("v0.66 locked GPU worker failed: %w; stderr: %s", runErr, detail)
	}
	return stdout.data, nil
}

// Test-only workerArgs support is private to this package. Production path
// passes nil: the approved GPU binary accepts the same stdin request as v0.62.
func runApprovedGPUWorkerWithSharedLockArgsV066(
	ctx context.Context, binaryPath, approvedSHA256 string,
	maxBytes int64, request []byte, lockDir string, workerArgs []string,
) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, fmt.Errorf("v0.66 missing active context")
	}
	sealed, err := newSealedGPUWorkerV066(binaryPath, approvedSHA256, maxBytes)
	if err != nil {
		return nil, err
	}
	defer sealed.Close()
	lock, err := tryAcquireSharedGPULockV066(lockDir)
	if err != nil {
		return nil, fmt.Errorf("v0.66 shared GPU reservation rejected: %w", err)
	}
	// No explicit LOCK_UN: child may still be alive after supervisor failure.
	defer lock.file.Close()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return runSealedGPUWorkerWithSharedLockV066(ctx, sealed, lock.file, request, workerArgs...)
}

func runApprovedGPUWorkerWithSharedLockV066(
	ctx context.Context, binaryPath, approvedSHA256 string,
	maxBytes int64, request []byte, lockDir string,
) ([]byte, error) {
	return runApprovedGPUWorkerWithSharedLockArgsV066(ctx, binaryPath, approvedSHA256, maxBytes, request, lockDir, nil)
}

// executeComputeGPUApprovedWithSharedLockV066 is an off-chain opt-in entrypoint.
// It does not modify the legacy v0.62/v0.63 paths, the consensus, or escrow.
// approvedSHA256 must come from an operator-controlled trusted policy.
func executeComputeGPUApprovedWithSharedLockV066(
	ctx context.Context, task usefulwork.Task, jobID, chainID, genesisHash string,
	worker *wallet.Wallet, binaryPath, approvedSHA256, lockDir string,
) (approvedGPUExecutionV066, error) {
	var empty approvedGPUExecutionV066
	if ctx == nil || ctx.Err() != nil || worker == nil {
		return empty, fmt.Errorf("v0.66 requires live context and worker identity")
	}
	if err := validateGPUComputeTaskV062(task); err != nil {
		return empty, fmt.Errorf("v0.66 invalid GPU task: %w", err)
	}
	contract, err := usefulwork.NewQuantizedMLComputeTaskContractV066(task, jobID, chainID, genesisHash, approvedSHA256)
	if err != nil {
		return empty, fmt.Errorf("v0.66 invalid contract: %w", err)
	}
	request, err := json.Marshal(struct {
		Task usefulwork.Task `json:"task"`
	}{Task: task})
	if err != nil {
		return empty, err
	}
	if len(request) > 512*1024 {
		return empty, fmt.Errorf("v0.66 request exceeds 512 KiB")
	}
	stdout, err := runApprovedGPUWorkerWithSharedLockV066(ctx, binaryPath, approvedSHA256, maxGPUExecutableBytesV066, request, lockDir)
	if err != nil {
		return empty, fmt.Errorf("v0.66 shared-locked CUDA launch rejected: %w", err)
	}
	predictions, err := parseGPUReceiptV062(task, stdout)
	if err != nil {
		return empty, fmt.Errorf("v0.66 locked GPU receipt rejected: %w", err)
	}
	proof, err := usefulwork.BuildComputeMLProofV2FromResult(task, jobID, chainID, genesisHash, worker, predictions)
	if err != nil {
		return empty, fmt.Errorf("v0.66 locked proof rejected: %w", err)
	}
	if err := usefulwork.VerifyQuantizedMLComputeTaskBindingV066(contract, proof, approvedSHA256); err != nil {
		return empty, fmt.Errorf("v0.66 locked proof binding rejected: %w", err)
	}
	return approvedGPUExecutionV066{Contract: contract, Proof: proof}, nil
}
