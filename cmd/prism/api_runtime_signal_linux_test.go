//go:build linux

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// This helper runs as a real child process.
func TestPrismAPIRuntimeSignalChild(t *testing.T) {
	if os.Getenv("PRISM_D5_SIGNAL_CHILD") != "1" {
		return
	}

	dir := os.Getenv("PRISM_D5_SIGNAL_DATA")
	address := os.Getenv("PRISM_D5_SIGNAL_ADDR")

	lock, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal(err)
	}

	server := &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			_, _ = io.WriteString(w, "ready")
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	clean, serveErr := servePrismAPIWithSignals(server)

	if serveErr != nil || !clean {
		// Deliberately retain the lock on unclean exit.
		t.Fatalf(
			"unclean HTTP shutdown: clean=%v err=%v",
			clean, serveErr,
		)
	}

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestPrismAPIRuntimeSignalShutdown(t *testing.T) {
	dir := t.TempDir()

	// Reserve an ephemeral loopback port for the child.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	address := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		executable,
		"-test.run=^TestPrismAPIRuntimeSignalChild$",
	)

	cmd.Env = append(
		os.Environ(),
		"PRISM_D5_SIGNAL_CHILD=1",
		"PRISM_D5_SIGNAL_DATA="+dir,
		"PRISM_D5_SIGNAL_ADDR="+address,
	)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	client := &http.Client{
		Timeout: 400 * time.Millisecond,
	}

	deadline := time.Now().Add(10 * time.Second)
	healthy := false

	for time.Now().Before(deadline) {
		response, err := client.Get(
			"http://" + address,
		)

		if err == nil {
			if response.StatusCode == http.StatusOK {
				healthy = true
			}
			_ = response.Body.Close()
		}

		if healthy {
			break
		}

		time.Sleep(25 * time.Millisecond)
	}

	if !healthy {
		t.Fatal("child HTTP server did not become ready")
	}

	// The live child must retain exclusive ownership.
	if other, err := acquirePrismAPIRuntimeLock(dir); err == nil {
		_ = other.Release()
		t.Fatal("second API acquired active lock")
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal("unable to send SIGTERM:", err)
	}

	waitErr := cmd.Wait()
	waited = true

	if waitErr != nil {
		t.Fatalf(
			"child did not shut down gracefully: %v\n%s",
			waitErr,
			output.String(),
		)
	}

	lockPath := filepath.Join(
		dir,
		prismAPIRuntimeLockName,
	)

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal(
			"graceful SIGTERM left runtime lock behind",
		)
	}

	restarted, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal(
			"restart after SIGTERM failed:",
			err,
		)
	}

	if err := restarted.Release(); err != nil {
		t.Fatal(err)
	}

	fmt.Println("SIGTERM graceful shutdown verified")
}
