//go:build linux && amd64

package computecloud

// Offline-only Linux/amd64 sealed worker primitive. This does NOT dispatch
// work, expose HTTP, sign a PoUW proof, or settle escrow. The Cloud offline
// runner executes this sealed snapshot via /proc/self/fd/3.

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	cloudMemfdCreateAMD64V1       = 319
	cloudMFDCloexecV1             = 0x0001
	cloudMFDAllowSealingV1        = 0x0002
	cloudFAddSealsV1              = 1033
	cloudFGetSealsV1              = 1034
	cloudFSealSealV1              = 0x0001
	cloudFSealShrinkV1            = 0x0002
	cloudFSealGrowV1              = 0x0004
	cloudFSealWriteV1             = 0x0008
	cloudRequiredSealsV1          = cloudFSealSealV1 | cloudFSealShrinkV1 | cloudFSealGrowV1 | cloudFSealWriteV1
	cloudMaxWorkerBytesV1   int64 = 256 << 20
)

// newSealedCloudGPUWorkerV1 captures exactly the independently approved bytes
// of a regular executable into a sealed memfd. The original path is not used
// for execution. Caller owns and must close the returned descriptor.
// SHA-256 pinning is not a substitute for a trusted approval policy or a
// process sandbox, and sealing is not hardware attestation.
func newSealedCloudGPUWorkerV1(sourcePath, approvedSHA256 string, maxBytes int64) (_ *os.File, err error) {
	if !filepath.IsAbs(sourcePath) || filepath.Clean(sourcePath) != sourcePath ||
		maxBytes <= 0 || maxBytes > cloudMaxWorkerBytesV1 {
		return nil, errors.New("invalid sealed Cloud worker path or size limit")
	}
	if len(approvedSHA256) != 64 {
		return nil, errors.New("approved Cloud worker hash must be 64 lowercase hex characters")
	}
	approved, e := hex.DecodeString(approvedSHA256)
	if e != nil || len(approved) != 32 || hex.EncodeToString(approved) != approvedSHA256 {
		return nil, errors.New("invalid approved Cloud worker SHA-256")
	}

	fd, e := syscall.Open(sourcePath, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, fmt.Errorf("open Cloud worker without symlinks: %w", e)
	}
	source := os.NewFile(uintptr(fd), "cloud-worker-source")
	defer source.Close()
	info, e := source.Stat()
	if e != nil {
		return nil, fmt.Errorf("fstat Cloud worker: %w", e)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes ||
		info.Mode().Perm()&0111 == 0 || info.Mode().Perm()&0022 != 0 {
		return nil, errors.New("Cloud worker must be a nonempty, executable, non-publicly-writable regular file within size limit")
	}

	name := append([]byte("prism-cloud-v1-approved-worker"), 0)
	memfd, _, errno := syscall.Syscall(cloudMemfdCreateAMD64V1,
		uintptr(unsafe.Pointer(&name[0])), uintptr(cloudMFDCloexecV1|cloudMFDAllowSealingV1), 0)
	if errno != 0 {
		return nil, fmt.Errorf("Cloud worker memfd_create unavailable: %w", errno)
	}
	sealed := os.NewFile(memfd, "cloud-sealed-worker")
	defer func() {
		if err != nil {
			_ = sealed.Close()
		}
	}()
	hash := sha256.New()
	copied, e := io.Copy(io.MultiWriter(sealed, hash), io.LimitReader(source, maxBytes+1))
	if e != nil {
		return nil, fmt.Errorf("copy Cloud worker snapshot: %w", e)
	}
	if copied != info.Size() || copied <= 0 || copied > maxBytes {
		return nil, errors.New("Cloud worker changed or exceeded byte limit during snapshot")
	}
	if subtle.ConstantTimeCompare(hash.Sum(nil), approved) != 1 {
		return nil, errors.New("Cloud worker snapshot does not match approved SHA-256")
	}
	if _, e = sealed.Seek(0, io.SeekStart); e != nil {
		return nil, fmt.Errorf("rewind Cloud sealed worker: %w", e)
	}
	if _, _, errno = syscall.Syscall(syscall.SYS_FCNTL, sealed.Fd(), cloudFAddSealsV1, cloudRequiredSealsV1); errno != 0 {
		return nil, fmt.Errorf("seal Cloud worker: %w", errno)
	}
	seals, _, errno := syscall.Syscall(syscall.SYS_FCNTL, sealed.Fd(), cloudFGetSealsV1, 0)
	if errno != 0 || seals&cloudRequiredSealsV1 != cloudRequiredSealsV1 {
		return nil, fmt.Errorf("Cloud worker seals not confirmed: %v", errno)
	}
	return sealed, nil
}
