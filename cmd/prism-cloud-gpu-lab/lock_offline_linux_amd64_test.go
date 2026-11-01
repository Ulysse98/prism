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
	"testing"

	"prism/internal/computecloud"
)

// This test holds the cooperative GPU lock before invoking the REAL CLI run
// function, and uses only a tiny shell fixture. No worker should ever start.
func TestCloudStandaloneGPUCLIRejectsBusySharedLockV1(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	lockDir, err := computecloud.DefaultCloudGPULockDirV1()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := computecloud.TryAcquireGlobalGPULockV1(lockDir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	binary := filepath.Join(t.TempDir(), "would-run-worker")
	body := []byte("#!/bin/sh\nexit 73\n")
	if err := os.WriteFile(binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	sample := `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`
	args := []string{"-enable-local-gpu", "-gpu-bin", binary, "-sha256", hex.EncodeToString(digest[:])}
	var out bytes.Buffer
	err = run(context.Background(), args, strings.NewReader(sample), &out)
	if !errors.Is(err, computecloud.ErrGlobalGPUBusyV1) {
		t.Fatalf("CLI failed to reject busy shared lock: %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("busy GPU CLI emitted an accepted result")
	}
}
