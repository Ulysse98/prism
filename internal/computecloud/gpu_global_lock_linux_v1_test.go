//go:build linux

package computecloud

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGlobalGPULockV1ExclusionAndRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reservation")
	first, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if second, err := TryAcquireGlobalGPULockV1(dir); second != nil || !errors.Is(err, ErrGlobalGPUBusyV1) {
		if second != nil {
			_ = second.Release()
		}
		t.Fatalf("second acquisition must be busy: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	third, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	if err := third.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalGPULockV1IndependentDirectories(t *testing.T) {
	root := t.TempDir()
	a, err := TryAcquireGlobalGPULockV1(filepath.Join(root, "a"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Release()
	b, err := TryAcquireGlobalGPULockV1(filepath.Join(root, "b"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()
}

func TestGlobalGPULockV1RejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	t.Run("relative", func(t *testing.T) {
		if l, err := TryAcquireGlobalGPULockV1("relative"); err == nil {
			_ = l.Release()
			t.Fatal("relative accepted")
		}
	})
	t.Run("symlink directory", func(t *testing.T) {
		realDir := filepath.Join(root, "real")
		if err := os.Mkdir(realDir, 0700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(realDir, link); err != nil {
			t.Fatal(err)
		}
		if l, err := TryAcquireGlobalGPULockV1(link); err == nil {
			_ = l.Release()
			t.Fatal("symlink accepted")
		}
	})
	t.Run("public directory", func(t *testing.T) {
		path := filepath.Join(root, "public")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
		if l, err := TryAcquireGlobalGPULockV1(path); err == nil {
			_ = l.Release()
			t.Fatal("public dir accepted")
		}
	})
	t.Run("symlink file", func(t *testing.T) {
		path := filepath.Join(root, "symlink-file")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("other", filepath.Join(path, globalGPULockFileV1)); err != nil {
			t.Fatal(err)
		}
		if l, err := TryAcquireGlobalGPULockV1(path); err == nil {
			_ = l.Release()
			t.Fatal("symlink file accepted")
		}
	})
	t.Run("hardlink file", func(t *testing.T) {
		path := filepath.Join(root, "hardlink")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		original := filepath.Join(path, globalGPULockFileV1)
		if err := os.WriteFile(original, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(original, filepath.Join(path, "other-link")); err != nil {
			t.Fatal(err)
		}
		if l, err := TryAcquireGlobalGPULockV1(path); err == nil {
			_ = l.Release()
			t.Fatal("hardlinked lock accepted")
		}
	})
}

func TestGlobalGPULockV1CrossProcessCrashRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "crash-test")
	cmd := exec.Command(os.Args[0], "-test.run=^TestGlobalGPULockV1ChildHelper$")
	cmd.Env = append(os.Environ(), "PRISM_GPU_LOCK_TEST_HELPER_DIR="+dir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	// A real subprocess, not just two Go goroutines, holds flock.
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			ready <- "EOF"
		}
	}()
	select {
	case line := <-ready:
		if line != "READY" {
			t.Fatalf("child did not acquire: %s", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child lock acquisition timed out")
	}
	if l, err := TryAcquireGlobalGPULockV1(dir); l != nil || !errors.Is(err, ErrGlobalGPUBusyV1) {
		if l != nil {
			_ = l.Release()
		}
		t.Fatalf("lock not exclusive across processes: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // expected SIGKILL
	finished = true
	recovered, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		t.Fatalf("kernel failed to release flock after crash: %v", err)
	}
	defer recovered.Release()
	if _, err := os.Stat(filepath.Join(dir, globalGPULockFileV1)); err != nil {
		t.Fatalf("lock inode should remain in place: %v", err)
	}
}

func TestGlobalGPULockV1ChildHelper(t *testing.T) {
	dir := os.Getenv("PRISM_GPU_LOCK_TEST_HELPER_DIR")
	if dir == "" {
		return
	}
	if strings.Contains(dir, "\n") {
		os.Exit(8)
	}
	lock, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		os.Exit(9)
	}
	defer lock.Release()
	_, _ = os.Stdout.WriteString("READY\n")
	for {
		time.Sleep(time.Hour)
	}
}
