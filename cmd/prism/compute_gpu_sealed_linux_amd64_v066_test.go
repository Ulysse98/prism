//go:build linux && amd64

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func makeGPUFixtureV066(t *testing.T) (string, string) {
	t.Helper()
	source := "/bin/echo"
	blob, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("/bin/echo is required for Linux ELF test: %v", err)
	}
	path := filepath.Join(t.TempDir(), "gpu-fixture")
	if err := os.WriteFile(path, blob, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(blob)
	return path, hex.EncodeToString(digest[:])
}

func TestGPUSealedWorkerV066RunsApprovedELF(t *testing.T) {
	path, approved := makeGPUFixtureV066(t)
	got, err := runApprovedGPUWorkerV066(context.Background(), path, approved, maxGPUExecutableBytesV066, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("\n")) {
		t.Fatalf("unexpected stdout: %q", got)
	}
}

func TestGPUSealedWorkerV066RejectsMutations(t *testing.T) {
	path, approved := makeGPUFixtureV066(t)
	tests := []struct {
		name   string
		path   string
		digest string
		limit  int64
	}{
		{"bad-digest", path, strings.Repeat("0", 64), maxGPUExecutableBytesV066},
		{"uppercase-digest", path, strings.ToUpper(approved), maxGPUExecutableBytesV066},
		{"short-digest", path, "abcd", maxGPUExecutableBytesV066},
		{"relative-path", "relative-worker", approved, maxGPUExecutableBytesV066},
		{"zero-limit", path, approved, 0},
		{"oversized-limit", path, approved, maxGPUExecutableBytesV066 + 1},
		{"undersized-limit", path, approved, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newSealedGPUWorkerV066(tc.path, tc.digest, tc.limit); err == nil {
				t.Fatal("unsafe worker accepted")
			}
		})
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := newSealedGPUWorkerV066(link, approved, maxGPUExecutableBytesV066); err == nil {
		t.Fatal("symlink unexpectedly allowed")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newSealedGPUWorkerV066(path, approved, maxGPUExecutableBytesV066); err == nil {
		t.Fatal("nonexecutable worker unexpectedly allowed")
	}
}

func TestGPUSealedWorkerV066PinsBytesAcrossPathReplacement(t *testing.T) {
	path, approved := makeGPUFixtureV066(t)
	sealed, err := newSealedGPUWorkerV066(path, approved, maxGPUExecutableBytesV066)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()

	// The original path now points to invalid code, but only the previously
	// checked sealed ELF should execute.
	replacement := filepath.Join(filepath.Dir(path), "replacement")
	if err := os.WriteFile(replacement, []byte("bad binary content"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	got, err := runSealedGPUWorkerV066(context.Background(), sealed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\n" {
		t.Fatalf("expected pinned /bin/echo executable, got %q", got)
	}
	if _, err := runApprovedGPUWorkerV066(context.Background(), path, approved, maxGPUExecutableBytesV066, nil); err == nil {
		t.Fatal("new launch should reject replaced source")
	}
}

func TestGPUSealedWorkerV066IsWriteSealed(t *testing.T) {
	path, approved := makeGPUFixtureV066(t)
	sealed, err := newSealedGPUWorkerV066(path, approved, maxGPUExecutableBytesV066)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	_, err = sealed.WriteAt([]byte("M"), 0)
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("expected EPERM when modifying sealed worker, got %v", err)
	}
}

func TestGPUSealedWorkerV066RejectsCancelledContext(t *testing.T) {
	path, approved := makeGPUFixtureV066(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runApprovedGPUWorkerV066(ctx, path, approved, maxGPUExecutableBytesV066, nil); err == nil {
		t.Fatal("cancelled run accepted")
	}
}

func TestGPUSealedWorkerV066RejectsInPlaceChange(t *testing.T) {
	path, approved := makeGPUFixtureV066(t)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("BAD!"), 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := newSealedGPUWorkerV066(path, approved, maxGPUExecutableBytesV066); err == nil {
		t.Fatal("tampered bytes accepted")
	}
}

func TestGPUSealedWorkerV066RejectsNonRegular(t *testing.T) {
	dir := t.TempDir()
	digest := strings.Repeat("1", 64)
	if _, err := newSealedGPUWorkerV066(dir, digest, maxGPUExecutableBytesV066); err == nil {
		t.Fatal("directory accepted")
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := newSealedGPUWorkerV066(empty, digest, maxGPUExecutableBytesV066); err == nil {
		t.Fatal("empty file accepted")
	}
}

func TestGPUSealedWorkerV066RejectsUnsealedFile(t *testing.T) {
	path, _ := makeGPUFixtureV066(t)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := runSealedGPUWorkerV066(context.Background(), file, nil); err == nil {
		t.Fatal("unsealed file accepted for execution")
	}
}

func TestGPUSealedWorkerV066RejectsWorkerNonzeroExit(t *testing.T) {
	data, err := os.ReadFile("/bin/false")
	if err != nil {
		t.Skipf("/bin/false unavailable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "exit-failure-worker")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if _, err := runApprovedGPUWorkerV066(context.Background(), path, hex.EncodeToString(hash[:]), maxGPUExecutableBytesV066, nil); err == nil {
		t.Fatal("nonzero worker exit accepted")
	}
}
