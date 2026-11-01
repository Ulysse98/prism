//go:build linux

// prism-cloud-gpu-broker-lab performs one operator-authorized offline GPU task.
// It is NOT registered with the Prism Cloud HTTP server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"prism/internal/computecloud"
)

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	f := flag.NewFlagSet("prism-cloud-gpu-broker-lab", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var enable bool
	var gpuBin, sha, chainID, genesis, approvedID string
	var nodePort int
	f.BoolVar(&enable, "enable-local-broker-gpu", false, "explicit operator-only GPU opt-in")
	f.StringVar(&approvedID, "confirm-task-id", "", "operator-approved canonical task ID")
	f.StringVar(&gpuBin, "gpu-bin", "", "absolute CUDA worker path")
	f.StringVar(&sha, "sha256", "", "expected lowercase SHA-256 of worker")
	f.IntVar(&nodePort, "node-port", 0, "localhost Prism status API port")
	f.StringVar(&chainID, "chain-id", "", "expected Prism chain ID")
	f.StringVar(&genesis, "genesis-hash", "", "expected lowercase genesis hash")
	if err := f.Parse(args); err != nil {
		return errors.New("invalid offline broker flags")
	}
	if !enable || approvedID == "" || gpuBin == "" || sha == "" ||
		nodePort == 0 || chainID == "" || genesis == "" || f.NArg() != 0 {
		return errors.New("offline GPU broker requires explicit opt-in, task confirmation, pinned node identity and pinned GPU binary")
	}
	observer, err := computecloud.NewCloudNodeObserverV1(nodePort, chainID, genesis)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(input, computecloud.MaxSubmitBodyBytesV1+1))
	if err != nil {
		return err
	}
	if len(data) > computecloud.MaxSubmitBodyBytesV1 {
		return errors.New("offline broker workload exceeds maximum input size")
	}
	lockDir, err := computecloud.DefaultCloudGPULockDirV1()
	if err != nil {
		return err
	}
	result, err := computecloud.RunLocalBrokerGPULabWithGlobalLockV1(ctx, data, observer, approvedID, gpuBin, sha, lockDir)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Prism offline GPU broker refused:", err)
		os.Exit(1)
	}
}
