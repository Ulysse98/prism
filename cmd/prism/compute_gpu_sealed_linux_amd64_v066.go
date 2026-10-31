//go:build linux && amd64

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// Linux amd64 syscall/flag constants. Kept local to avoid a new module dependency.
const (
	memfdCreateAMD64V066            = 319
	mfdCloexecV066                  = 0x0001
	mfdAllowSealingV066             = 0x0002
	fAddSealsV066                   = 1033
	fGetSealsV066                   = 1034
	fSealSealV066                   = 0x0001
	fSealShrinkV066                 = 0x0002
	fSealGrowV066                   = 0x0004
	fSealWriteV066                  = 0x0008
	requiredSealsV066               = fSealSealV066 | fSealShrinkV066 | fSealGrowV066 | fSealWriteV066
	maxGPUExecutableBytesV066 int64 = 512 << 20
)

// newSealedGPUWorkerV066 captures the bytes of an absolute, regular executable
// into a Linux memfd, checks the captured bytes against an independently
// approved SHA-256, then applies immutable file seals before returning it.
// The source path is never used for execution. A changed source can at most
// cause rejection or produce the exact approved bytes in the sealed snapshot.
// This is a Linux/amd64 *off-chain lab* primitive, NOT hardware attestation.
func newSealedGPUWorkerV066(sourcePath, approvedSHA256 string, maxBytes int64) (_ *os.File, err error) {
	if !filepath.IsAbs(sourcePath) || maxBytes <= 0 || maxBytes > maxGPUExecutableBytesV066 {
		return nil, fmt.Errorf("v0.66 invalid absolute worker path or byte limit")
	}
	if len(approvedSHA256) != 64 {
		return nil, fmt.Errorf("v0.66 approved worker SHA-256 must be 64 lowercase hex characters")
	}
	approved, decodeErr := hex.DecodeString(approvedSHA256)
	if decodeErr != nil || hex.EncodeToString(approved) != approvedSHA256 {
		return nil, fmt.Errorf("v0.66 invalid approved worker SHA-256")
	}

	// O_NOFOLLOW rejects a final-component symlink at open time.
	fd, openErr := syscall.Open(sourcePath, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if openErr != nil {
		return nil, fmt.Errorf("v0.66 opening worker: %w", openErr)
	}
	source := os.NewFile(uintptr(fd), "v066-worker-source")
	defer source.Close()
	sourceInfo, statErr := source.Stat()
	if statErr != nil {
		return nil, fmt.Errorf("v0.66 fstat worker: %w", statErr)
	}
	if !sourceInfo.Mode().IsRegular() || sourceInfo.Size() <= 0 || sourceInfo.Size() > maxBytes || sourceInfo.Mode().Perm()&0111 == 0 {
		return nil, fmt.Errorf("v0.66 worker must be a nonempty, executable regular file within size limit")
	}

	// memfd_create(MFD_CLOEXEC|MFD_ALLOW_SEALING). Child fd 3 is explicitly
	// provided via exec.Cmd.ExtraFiles after all seals have been installed.
	name := append([]byte("prism-v066-approved-worker"), 0)
	memfd, _, errno := syscall.Syscall(
		memfdCreateAMD64V066,
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(mfdCloexecV066|mfdAllowSealingV066),
		0,
	)
	if errno != 0 {
		return nil, fmt.Errorf("v0.66 memfd_create unavailable: %w", errno)
	}
	sealed := os.NewFile(memfd, "v066-sealed-worker")
	defer func() {
		if err != nil {
			_ = sealed.Close()
		}
	}()

	hash := sha256.New()
	copied, copyErr := io.Copy(io.MultiWriter(sealed, hash), io.LimitReader(source, maxBytes+1))
	if copyErr != nil {
		return nil, fmt.Errorf("v0.66 copy approved worker: %w", copyErr)
	}
	if copied != sourceInfo.Size() || copied <= 0 || copied > maxBytes {
		return nil, fmt.Errorf("v0.66 worker changed or exceeded byte limit while copying")
	}
	if subtle.ConstantTimeCompare(hash.Sum(nil), approved) != 1 {
		return nil, fmt.Errorf("v0.66 captured worker bytes differ from approved SHA-256")
	}
	if _, seekErr := sealed.Seek(0, io.SeekStart); seekErr != nil {
		return nil, fmt.Errorf("v0.66 rewind memfd: %w", seekErr)
	}
	_, _, errno = syscall.Syscall(syscall.SYS_FCNTL, memfd, fAddSealsV066, requiredSealsV066)
	if errno != 0 {
		return nil, fmt.Errorf("v0.66 failed to seal worker bytes: %w", errno)
	}
	currentSeals, _, errno := syscall.Syscall(syscall.SYS_FCNTL, memfd, fGetSealsV066, 0)
	if errno != 0 || currentSeals&requiredSealsV066 != requiredSealsV066 {
		return nil, fmt.Errorf("v0.66 worker sealing not confirmed: %v", errno)
	}
	return sealed, nil
}

// runSealedGPUWorkerV066 runs only the sealed snapshot via inherited child FD 3.
// Keeping the parent fd open across Run ensures the snapshot's lifetime.
func runSealedGPUWorkerV066(ctx context.Context, sealed *os.File, request []byte) ([]byte, error) {
	if ctx == nil || sealed == nil {
		return nil, fmt.Errorf("v0.66 invalid worker context or sealed file")
	}
	// Refuse arbitrary or writable fds: sealing is irreversible once installed.
	currentSeals, _, errno := syscall.Syscall(syscall.SYS_FCNTL, sealed.Fd(), fGetSealsV066, 0)
	if errno != 0 || currentSeals&requiredSealsV066 != requiredSealsV066 {
		return nil, fmt.Errorf("v0.66 execution requires a fully sealed memfd: %v", errno)
	}
	// Preserve the legacy one-shot maximum runtime even when the caller
	// supplies a context without a deadline.
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// The child gets the memfd as descriptor 3, independently of the pathname
	// of the original worker. /proc must be mounted in the worker namespace.
	cmd := exec.CommandContext(runCtx, "/proc/self/fd/3")
	cmd.ExtraFiles = []*os.File{sealed}
	if err := configureGPUWorkerProcessV062(cmd); err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewReader(request)
	cmd.WaitDelay = 2 * time.Second
	stdout := &boundedGPUOutputV062{limit: 128 * 1024}
	stderr := &boundedGPUOutputV062{limit: 16 * 1024}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	if runCtx.Err() != nil {
		return nil, fmt.Errorf("v0.66 worker timed out or cancelled: %w", runCtx.Err())
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, fmt.Errorf("v0.66 worker output exceeded bounded limit")
	}
	if runErr != nil {
		detail := string(stderr.data)
		if len(detail) > 2000 {
			detail = detail[:2000]
		}
		return nil, fmt.Errorf("v0.66 sealed worker failed: %w; stderr: %s", runErr, detail)
	}
	return stdout.data, nil
}

// runApprovedGPUWorkerV066 snapshots, validates, seals, then runs the worker.
// approvedSHA256 MUST be from an independently trusted allowlist/policy;
// this function does not validate the policy, sign a proof, or settle escrow.
func runApprovedGPUWorkerV066(ctx context.Context, sourcePath, approvedSHA256 string, maxBytes int64, request []byte) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, fmt.Errorf("v0.66 missing or cancelled context")
	}
	sealed, err := newSealedGPUWorkerV066(sourcePath, approvedSHA256, maxBytes)
	if err != nil {
		return nil, err
	}
	defer sealed.Close()
	return runSealedGPUWorkerV066(ctx, sealed, request)
}
