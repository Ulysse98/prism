//go:build linux

package main

// Offline standalone demonstration of a cooperative process-wide CUDA lock.
// Does not invoke CUDA, Prism API, a marketplace or any signing routine.
import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"prism/internal/computecloud"
)

func run(args []string, out, errout io.Writer) int {
	flags := flag.NewFlagSet("prism-cloud-gpu-lock-lab", flag.ContinueOnError)
	flags.SetOutput(errout)
	enabled := flags.Bool("enable-global-gpu-lock", false, "explicitly authorize this offline lock probe")
	directory := flags.String("lock-dir", "", "private absolute lock directory (default $HOME/.prism-cloud-gpu-lock-v1)")
	duration := flags.Duration("hold-for", 3*time.Second, "time to hold the lock, max 30 seconds")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(flags.Args()) != 0 || !*enabled {
		fmt.Fprintln(errout, "refused: requires explicit opt-in -enable-global-gpu-lock")
		return 2
	}
	if *duration <= 0 || *duration > 30*time.Second {
		fmt.Fprintln(errout, "refused: invalid hold-for duration")
		return 2
	}
	if *directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(errout, "refused: no home directory")
			return 2
		}
		*directory = filepath.Join(home, ".prism-cloud-gpu-lock-v1")
	}
	lock, err := computecloud.TryAcquireGlobalGPULockV1(*directory)
	if err != nil {
		if errors.Is(err, computecloud.ErrGlobalGPUBusyV1) {
			fmt.Fprintln(errout, "GPU_BUSY: another cooperative process owns the GPU lock")
			return 3
		}
		fmt.Fprintln(errout, "lock refused:", err)
		return 2
	}
	defer lock.Release()
	encoder := json.NewEncoder(out)
	if err := encoder.Encode(struct {
		Mode                string `json:"mode"`
		Status              string `json:"status"`
		PID                 int    `json:"pid"`
		HTTPDispatchEnabled bool   `json:"http_dispatch_enabled"`
		CUDAExecuted        bool   `json:"cuda_executed"`
	}{"offline_gpu_global_lock_lab", "lock_acquired", os.Getpid(), false, false}); err != nil {
		fmt.Fprintln(errout, "output failed:", err)
		return 2
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	timer := time.NewTimer(*duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-stop:
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
