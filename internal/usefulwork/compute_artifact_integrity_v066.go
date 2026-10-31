package usefulwork

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MaxComputeArtifactBytesV066 caps file hashing to a fixed, bounded amount.
// This is an OFF-CHAIN preflight limit, not a consensus workload limit.
const MaxComputeArtifactBytesV066 int64 = 512 << 20

// ComputeArtifactFileSHA256V066 hashes the actual bytes of a regular local file.
// It rejects relative paths, final-component symlinks, non-regular files,
// empty files, file identity changes between Lstat/Open, and changes to file
// size or mtime observed during reading. It never executes the file.
//
// The containing directories MUST be trusted. This check does not guarantee
// immutable bytes between preflight and subsequent worker execution; a caller
// requiring that guarantee must execute a securely pinned/copied artifact.
func ComputeArtifactFileSHA256V066(path string, maxBytes int64) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("v0.66 artifact path must be absolute")
	}
	if maxBytes <= 0 || maxBytes > MaxComputeArtifactBytesV066 {
		return "", fmt.Errorf("v0.66 artifact max size out of bounds")
	}

	before, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("stat v0.66 artifact: %w", err)
	}
	if !before.Mode().IsRegular() {
		return "", fmt.Errorf("v0.66 artifact must be a regular file, not a symlink")
	}
	if before.Size() <= 0 || before.Size() > maxBytes {
		return "", fmt.Errorf("v0.66 artifact empty or exceeds size limit")
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open v0.66 artifact: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("fstat v0.66 artifact: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) ||
		opened.Size() != before.Size() || !opened.ModTime().Equal(before.ModTime()) {
		return "", fmt.Errorf("v0.66 artifact changed before read")
	}

	sha := sha256.New()
	n, err := io.Copy(sha, io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("hash v0.66 artifact: %w", err)
	}
	if n <= 0 || n > maxBytes {
		return "", fmt.Errorf("v0.66 artifact empty or exceeds size limit while reading")
	}
	after, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("final fstat v0.66 artifact: %w", err)
	}
	if n != opened.Size() || after.Size() != opened.Size() ||
		!after.ModTime().Equal(opened.ModTime()) || !os.SameFile(opened, after) {
		return "", fmt.Errorf("v0.66 artifact changed during read")
	}
	return hex.EncodeToString(sha.Sum(nil)), nil
}

// VerifyComputeArtifactFileV066 checks actual file bytes against a digest
// provided by an INDEPENDENT approved policy (not the request or proof).
// Comparison is fail-closed; verifying a file is NOT GPU execution attestation.
func VerifyComputeArtifactFileV066(path, approvedSHA256 string, maxBytes int64) error {
	if !validSHA256HexV066(approvedSHA256) {
		return fmt.Errorf("v0.66 approved artifact SHA-256 is invalid")
	}
	actual, err := ComputeArtifactFileSHA256V066(path, maxBytes)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(approvedSHA256)) != 1 {
		return fmt.Errorf("v0.66 artifact bytes do not match approved digest")
	}
	return nil
}

// VerifyQuantizedMLComputeTaskBindingWithArtifactV066 ties the existing v0.66
// contract/proof bridge to a local artifact byte check. It is off-chain only,
// with NO escrow authority, GPU execution attestation or process launch.
// approvedSHA256 must come from a trusted policy external to the contract.
func VerifyQuantizedMLComputeTaskBindingWithArtifactV066(
	contract ComputeTaskContractV066,
	proof Proof,
	artifactPath, approvedSHA256 string,
	maxBytes int64,
) error {
	if err := VerifyComputeArtifactFileV066(artifactPath, approvedSHA256, maxBytes); err != nil {
		return fmt.Errorf("v0.66 artifact preflight failed: %w", err)
	}
	if err := VerifyQuantizedMLComputeTaskBindingV066(contract, proof, approvedSHA256); err != nil {
		return fmt.Errorf("v0.66 artifact-bound proof rejected: %w", err)
	}
	return nil
}
