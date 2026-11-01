//go:build linux

package computecloud

import (
	"path/filepath"
	"testing"
)

func TestCloudDefaultGlobalLockDirV1(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := DefaultCloudGPULockDirV1()
	if err != nil || got != filepath.Join(home, ".prism-cloud-gpu-lock-v1") {
		t.Fatalf("unexpected shared lock dir %q: %v", got, err)
	}
	lock, err := TryAcquireGlobalGPULockV1(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", "relative-home")
	if _, err := DefaultCloudGPULockDirV1(); err == nil {
		t.Fatal("accepted relative HOME for GPU global lock")
	}
}
