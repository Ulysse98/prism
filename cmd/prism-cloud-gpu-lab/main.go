//go:build linux

// prism-cloud-gpu-lab is an explicit, offline-only CUDA qualification CLI.
// It must never be imported by the SaaS HTTP server.
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
	flags := flag.NewFlagSet("prism-cloud-gpu-lab", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var gpuBin, sha string
	var enable bool
	flags.StringVar(&gpuBin, "gpu-bin", "", "absolute trusted GPU worker executable")
	flags.StringVar(&sha, "sha256", "", "expected lowercase worker binary SHA256")
	flags.BoolVar(&enable, "enable-local-gpu", false, "explicitly execute offline CUDA lab")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid offline GPU lab flags")
	}
	if !enable || gpuBin == "" || sha == "" || flags.NArg() != 0 {
		return errors.New("offline GPU lab requires -enable-local-gpu -gpu-bin ABSOLUTE_PATH -sha256 EXPECTED_SHA256")
	}
	data, err := io.ReadAll(io.LimitReader(input, computecloud.MaxSubmitBodyBytesV1+1))
	if err != nil {
		return err
	}
	if len(data) > computecloud.MaxSubmitBodyBytesV1 {
		return errors.New("input body exceeds Cloud v1 limit")
	}
	result, err := computecloud.RunLocalGPULabV1(ctx, data, gpuBin, sha)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Prism offline GPU lab refused:", err)
		os.Exit(1)
	}
}
