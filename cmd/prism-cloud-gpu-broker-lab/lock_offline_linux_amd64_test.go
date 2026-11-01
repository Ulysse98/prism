//go:build linux && amd64

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"prism/internal/computecloud"
)

// Mock loopback node status only. The GPU lock is held so the fixture worker
// is refused before execution. This checks the broker CLI actually takes the
// locked path and never registers any HTTP compute route.
func TestCloudStandaloneBrokerCLIRejectsBusySharedLockV1(t *testing.T) {
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

	const chain = "prism-local-2f3"
	genesis := strings.Repeat("a", 64)
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/status" {
			t.Errorf("unexpected node request %s %s", r.Method, r.URL.Path)
			http.Error(w, "invalid", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version":"0.66.0","chainId":%q,"genesisHash":%q,"height":1,"chainValid":true}`, chain, genesis)
	}))
	defer node.Close()
	host, portString, err := net.SplitHostPort(strings.TrimPrefix(node.URL, "http://"))
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("unsafe mock observer: %v %s", err, host)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "would-run-worker")
	body := []byte("#!/bin/sh\nexit 73\n")
	if err := os.WriteFile(binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	sample := `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`
	task, err := computecloud.ParseSubmitV1([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"-enable-local-broker-gpu", "-confirm-task-id", task.ID,
		"-gpu-bin", binary, "-sha256", hex.EncodeToString(digest[:]),
		"-node-port", strconv.Itoa(port), "-chain-id", chain,
		"-genesis-hash", genesis,
	}
	var out bytes.Buffer
	err = run(context.Background(), args, strings.NewReader(sample), &out)
	if !errors.Is(err, computecloud.ErrGlobalGPUBusyV1) {
		t.Fatalf("broker CLI failed to reject busy shared lock: %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("busy broker emitted an accepted result")
	}
}
