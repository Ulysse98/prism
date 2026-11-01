//go:build linux

package computecloud

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This helper is an ordinary executable process, not a CUDA simulation.
// Its inherited fd3 must stay open for the duration of the test.
func TestGPUInheritedLockWorkerHelperV1(t *testing.T) {
	if os.Getenv("PRISM_LOCK_INHERITED_WORKER_V1") != "1" {
		return
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(3, &stat); err != nil {
		os.Exit(78)
	}
	fmt.Fprintln(os.Stdout, "WORKER_READY")
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestGPUInheritedLockSupervisorHelperV1(t *testing.T) {
	dir := os.Getenv("PRISM_LOCK_INHERITED_SUPERVISOR_V1")
	if dir == "" {
		return
	}
	lock, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		os.Exit(79)
	}
	worker := exec.Command(os.Args[0], "-test.run=^TestGPUInheritedLockWorkerHelperV1$")
	worker.Env = append(os.Environ(), "PRISM_LOCK_INHERITED_WORKER_V1=1")
	worker.ExtraFiles = []*os.File{lock.file}
	worker.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := worker.StdoutPipe()
	if err != nil {
		os.Exit(80)
	}
	if err := worker.Start(); err != nil {
		os.Exit(81)
	}
	// The child must prove it received fd3 before the supervisor advertises it.
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "WORKER_READY" {
		os.Exit(82)
	}
	fmt.Printf("WORKER_PID=%d\n", worker.Process.Pid)
	// Intentionally never Release: the parent test sends SIGKILL.
	_ = lock
	time.Sleep(30 * time.Second)
}

func TestGPUInheritedLockSurvivesSupervisorKillV1(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inherited-lock")
	supervisor := exec.Command(os.Args[0], "-test.run=^TestGPUInheritedLockSupervisorHelperV1$")
	supervisor.Env = append(os.Environ(), "PRISM_LOCK_INHERITED_SUPERVISOR_V1="+dir)
	stdout, err := supervisor.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(); err != nil {
		t.Fatal(err)
	}
	supervisorDone := false
	workerPID := 0
	defer func() {
		if !supervisorDone {
			_ = supervisor.Process.Kill()
			_ = supervisor.Wait()
		}
		if workerPID > 0 {
			_ = syscall.Kill(workerPID, syscall.SIGKILL)
		}
	}()
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		ready <- strings.TrimSpace(line)
	}()
	var line string
	select {
	case line = <-ready:
	case <-time.After(8 * time.Second):
		t.Fatal("supervisor launch timeout")
	}
	if !strings.HasPrefix(line, "WORKER_PID=") {
		t.Fatalf("no worker handshake: %q", line)
	}
	workerPID, err = strconv.Atoi(strings.TrimPrefix(line, "WORKER_PID="))
	if err != nil || workerPID < 2 {
		t.Fatalf("invalid worker PID: %q", line)
	}

	// Parent and child share one open-file description: never use LOCK_UN.
	if err := supervisor.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = supervisor.Wait()
	supervisorDone = true
	if l, e := TryAcquireGlobalGPULockV1(dir); l != nil || !errors.Is(e, ErrGlobalGPUBusyV1) {
		if l != nil {
			_ = l.Release()
		}
		t.Fatalf("orphan worker did not retain GPU flock: %v", e)
	}
	if err := syscall.Kill(workerPID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		l, e := TryAcquireGlobalGPULockV1(dir)
		if e == nil {
			if err := l.Release(); err != nil {
				t.Fatal(err)
			}
			return
		}
		if !errors.Is(e, ErrGlobalGPUBusyV1) {
			t.Fatal(e)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("kernel did not release inherited lock after worker exit")
}

func TestGPUInheritedLockCloseOnlyV1(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "close-only")
	lock, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestGPUInheritedLockWorkerHelperV1$")
	cmd.Env = append(os.Environ(), "PRISM_LOCK_INHERITED_WORKER_V1=1")
	cmd.ExtraFiles = []*os.File{lock.file}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "WORKER_READY" {
		t.Fatalf("worker failed to inherit FD: %q %v", line, err)
	}
	if err := lock.ClosePreservingInheritedLockV1(); err != nil {
		t.Fatal(err)
	}
	if err := lock.ClosePreservingInheritedLockV1(); err != nil {
		t.Fatal(err)
	}
	if l, e := TryAcquireGlobalGPULockV1(dir); l != nil || !errors.Is(e, ErrGlobalGPUBusyV1) {
		if l != nil {
			_ = l.Release()
		}
		t.Fatalf("close-only incorrectly unlocked: %v", e)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	recovered, err := TryAcquireGlobalGPULockV1(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Release()
}
