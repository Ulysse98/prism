//go:build linux && amd64

package computecloud

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cloudSealedHashV1(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestCloudSealedWorkerV1PinsBytesAcrossPathReplacement(t *testing.T) {
	dir := t.TempDir()
	original := []byte("approved immutable worker bytes")
	source := filepath.Join(dir, "worker")
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(source, original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte("unapproved replacement"), 0700); err != nil {
		t.Fatal(err)
	}
	sealed, err := newSealedCloudGPUWorkerV1(source, cloudSealedHashV1(original), 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	if err := os.Rename(replacement, source); err != nil {
		t.Fatal(err)
	}
	if _, err := sealed.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	snapshot, err := io.ReadAll(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != string(original) {
		t.Fatalf("approved bytes not pinned: %q", snapshot)
	}
	if _, err := sealed.Write([]byte("altered")); err == nil {
		t.Fatal("sealed worker unexpectedly writable")
	}
}

func TestCloudSealedWorkerV1RejectsWrongHashSymlinkAndLimits(t *testing.T) {
	dir := t.TempDir()
	original := []byte("original worker bytes")
	source := filepath.Join(dir, "worker")
	if err := os.WriteFile(source, original, 0700); err != nil {
		t.Fatal(err)
	}
	approve := cloudSealedHashV1(original)
	for _, tc := range []struct {
		name, path, hash string
		limit            int64
	}{
		{"wrong hash", source, strings.Repeat("0", 64), 1024},
		{"noncanonical hash", source, strings.ToUpper(approve), 1024},
		{"size limit", source, approve, 4},
		{"relative path", "worker", approve, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := newSealedCloudGPUWorkerV1(tc.path, tc.hash, tc.limit)
			if f != nil {
				f.Close()
			}
			if err == nil {
				t.Fatal("unsafe worker accepted")
			}
		})
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if f, err := newSealedCloudGPUWorkerV1(link, approve, 1024); err == nil {
		f.Close()
		t.Fatal("symlink accepted")
	}
	if err := os.Chmod(source, 0666); err != nil {
		t.Fatal(err)
	}
	if f, err := newSealedCloudGPUWorkerV1(source, approve, 1024); err == nil {
		f.Close()
		t.Fatal("non-executable publicly writable worker accepted")
	}
}

// Build our own tiny ELF fixture rather than copying /bin/echo. Some Ubuntu
// coreutils builds reject memfd argv[0], even though direct ELF execution works.
func TestCloudSealedWorkerV1ExecutesOnlyApprovedELF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fixture.go")
	bin := filepath.Join(dir, "fixture")
	if err := os.WriteFile(src, []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"cloud-sealed-ok\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-trimpath", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v\n%s", err, out)
	}
	bytes, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := newSealedCloudGPUWorkerV1(bin, cloudSealedHashV1(bytes), cloudMaxWorkerBytesV1)
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	run := exec.Command("/proc/self/fd/3")
	run.ExtraFiles = []*os.File{sealed}
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("sealed ELF execution failed: %v\n%s", err, out)
	}
	if string(out) != "cloud-sealed-ok\n" {
		t.Fatalf("unexpected sealed output: %q", out)
	}
}
