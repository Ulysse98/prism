//go:build linux

// Operator-only offline queue connecting the Prism broker and GPU scheduler.
// Never exposed as a route in prism-cloud serve; never signs or settles.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"prism/internal/computecloud"
)

type batchResult struct {
	Mode                string                                     `json:"mode"`
	SerialPerScheduler  bool                                       `json:"serial_per_scheduler"`
	Results             []computecloud.CloudLocalBrokerGPUResultV1 `json:"results"`
	HTTPDispatchEnabled bool                                       `json:"http_dispatch_enabled"`
	EscrowEnabled       bool                                       `json:"escrow_enabled"`
	ProofsAvailable     bool                                       `json:"proofs_available"`
}

func decodeApprovedWork(data []byte, confirmations string) ([]computecloud.ApprovedLocalGPUWorkV1, error) {
	if len(data) > 4*(computecloud.MaxSubmitBodyBytesV1+1)+1024 {
		return nil, errors.New("GPU queue input exceeds limit")
	}
	var bodies []json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&bodies); err != nil {
		return nil, errors.New("GPU queue requires a JSON array of workloads")
	}
	var remainder any
	if err := decoder.Decode(&remainder); !errors.Is(err, io.EOF) {
		return nil, errors.New("GPU queue has trailing JSON")
	}
	ids := strings.Split(confirmations, ",")
	if len(bodies) < 2 || len(bodies) > 4 || len(ids) != len(bodies) {
		return nil, errors.New("GPU queue requires 2..4 workloads and one separately approved task ID each")
	}
	works := make([]computecloud.ApprovedLocalGPUWorkV1, len(bodies))
	for i := range bodies {
		if len(bodies[i]) > computecloud.MaxSubmitBodyBytesV1 {
			return nil, fmt.Errorf("workload %d too large", i+1)
		}
		works[i] = computecloud.ApprovedLocalGPUWorkV1{Body: append([]byte(nil), bodies[i]...), ApprovedTaskID: ids[i]}
	}
	return works, nil
}

func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	f := flag.NewFlagSet("prism-cloud-gpu-queue-lab", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var enabled bool
	var approvals, gpuBin, pin, chain, genesis string
	var port int
	f.BoolVar(&enabled, "enable-local-queue-gpu", false, "explicit operator-only GPU queue opt-in")
	f.StringVar(&approvals, "confirm-task-ids", "", "comma-separated exact canonical task IDs, in JSON array order")
	f.StringVar(&gpuBin, "gpu-bin", "", "absolute CUDA binary path")
	f.StringVar(&pin, "sha256", "", "pinned CUDA binary SHA-256")
	f.IntVar(&port, "node-port", 0, "localhost Prism status port")
	f.StringVar(&chain, "chain-id", "", "expected Prism chain ID")
	f.StringVar(&genesis, "genesis-hash", "", "expected lowercase 64-char genesis hash")
	if err := f.Parse(args); err != nil {
		return errors.New("invalid offline queue flags")
	}
	if !enabled || approvals == "" || gpuBin == "" || pin == "" || port == 0 || chain == "" || genesis == "" || f.NArg() != 0 {
		return errors.New("offline GPU queue requires explicit opt-in, per-task confirmation and pinned worker/node")
	}
	// Fully read bounded input and verify every approval BEFORE dialing a
	// node or invoking a CUDA process.
	max := int64(4*(computecloud.MaxSubmitBodyBytesV1+1) + 1025)
	data, err := io.ReadAll(io.LimitReader(in, max))
	if err != nil {
		return err
	}
	works, err := decodeApprovedWork(data, approvals)
	if err != nil {
		return err
	}
	observer, err := computecloud.NewCloudNodeObserverV1(port, chain, genesis)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("GPU lock home unavailable: %w", err)
	}
	// All cooperating local queues use one stable lock path.
	lockDir := filepath.Join(home, ".prism-cloud-gpu-lock-v1")
	runner, err := computecloud.NewOperatorGPUQueueRunnerWithGlobalLockV1(observer, gpuBin, pin, lockDir)
	if err != nil {
		return err
	}
	results, err := computecloud.RunApprovedLocalGPUQueueV1(ctx, works, runner)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(batchResult{
		Mode: "operator_local_gpu_queue_lab", SerialPerScheduler: true, Results: results,
		HTTPDispatchEnabled: false, EscrowEnabled: false, ProofsAvailable: false,
	})
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Prism offline GPU queue refused:", err)
		os.Exit(1)
	}
}
