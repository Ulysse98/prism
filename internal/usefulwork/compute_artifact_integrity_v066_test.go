package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func artifactFixtureV066(t *testing.T, data []byte) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worker.bin")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	sha := sha256.Sum256(data)
	return path, hex.EncodeToString(sha[:])
}

func TestComputeArtifactFileV066HashAndApproval(t *testing.T) {
	const data = "prism-v066-test-worker-bytes"
	path, expected := artifactFixtureV066(t, []byte(data))
	actual, err := ComputeArtifactFileSHA256V066(path, 1024)
	if err != nil || actual != expected {
		t.Fatalf("artifact hash mismatch: %q %v", actual, err)
	}
	if err := VerifyComputeArtifactFileV066(path, expected, 1024); err != nil {
		t.Fatalf("approved artifact rejected: %v", err)
	}
}

func TestComputeArtifactFileV066RejectBadInputs(t *testing.T) {
	path, hash := artifactFixtureV066(t, []byte("trusted worker"))
	badCases := []struct {
		name, file, expected string
		limit                int64
	}{
		{"wrong-approved-hash", path, strings.Repeat("f", 64), 1024},
		{"malformed-approved-hash", path, "bad", 1024},
		{"uppercase-approved-hash", path, strings.ToUpper(hash), 1024},
		{"zero-limit", path, hash, 0},
		{"negative-limit", path, hash, -1},
		{"excessive-limit", path, hash, MaxComputeArtifactBytesV066 + 1},
		{"too-small-limit", path, hash, 1},
		{"relative-path", "worker.bin", hash, 1024},
		{"missing-file", filepath.Join(t.TempDir(), "missing"), hash, 1024},
		{"directory", t.TempDir(), hash, 1024},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyComputeArtifactFileV066(tc.file, tc.expected, tc.limit); err == nil {
				t.Fatal("invalid artifact accepted")
			}
		})
	}
	empty, _ := artifactFixtureV066(t, nil)
	if _, err := ComputeArtifactFileSHA256V066(empty, 1024); err == nil {
		t.Fatal("empty file accepted")
	}
	link := filepath.Join(t.TempDir(), "worker-link")
	if err := os.Symlink(path, link); err == nil {
		if _, err := ComputeArtifactFileSHA256V066(link, 1024); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}

func TestComputeArtifactFileV066RejectMutation(t *testing.T) {
	path, originalSHA := artifactFixtureV066(t, []byte("original"))
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyComputeArtifactFileV066(path, originalSHA, 1024); err == nil {
		t.Fatal("mutated artifact approved")
	}
}

func TestQuantizedMLComputeBindingWithArtifactV066(t *testing.T) {
	path, sha := artifactFixtureV066(t, []byte("compiled-worker-for-lab"))
	contract, proof, _ := quantizedMLBindingFixtureV066(t)
	contract.ArtifactHash = sha
	const limit int64 = 1024
	if err := VerifyQuantizedMLComputeTaskBindingWithArtifactV066(
		contract, proof, path, sha, limit,
	); err != nil {
		t.Fatalf("artifact-bound valid proof rejected: %v", err)
	}
	t.Run("unapproved-file", func(t *testing.T) {
		if err := VerifyQuantizedMLComputeTaskBindingWithArtifactV066(
			contract, proof, path, strings.Repeat("a", 64), limit,
		); err == nil {
			t.Fatal("unapproved artifact accepted")
		}
	})
	t.Run("contract-only-hash", func(t *testing.T) {
		c := contract
		c.ArtifactHash = strings.Repeat("a", 64)
		if err := VerifyQuantizedMLComputeTaskBindingWithArtifactV066(
			c, proof, path, sha, limit,
		); err == nil {
			t.Fatal("wrong contract artifact accepted")
		}
	})
	t.Run("tampered-proof", func(t *testing.T) {
		p := proof
		p.Signature = "00"
		if err := VerifyQuantizedMLComputeTaskBindingWithArtifactV066(
			contract, p, path, sha, limit,
		); err == nil {
			t.Fatal("invalid signed proof accepted")
		}
	})
	t.Run("changed-binary", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("tampered-worker-for-lab"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := VerifyQuantizedMLComputeTaskBindingWithArtifactV066(
			contract, proof, path, sha, limit,
		); err == nil {
			t.Fatal("tampered executable accepted")
		}
	})
}
