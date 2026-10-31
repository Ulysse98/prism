//go:build linux && amd64

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// approvedGPUExecutionV066 is an off-chain result bundle; the contract is
// UNSIGNED and not part of the PoUW v2 worker signature. No settlement authority.
type approvedGPUExecutionV066 struct {
	Contract usefulwork.ComputeTaskContractV066
	Proof    usefulwork.Proof
}

// executeComputeGPUApprovedV066 is an EXPLICIT OPT-IN alternative to the
// legacy one-shot launch. No existing CLI, session, consensus, or escrow route
// calls it automatically. approvedSHA256 MUST come from an independently
// trusted, operator-controlled allowlist; never from a task, receipt or client.
//
// The 1D.1 launcher copies the executable into a sealed Linux memfd, compares
// its real bytes with the approved hash, and executes that exact snapshot.
// The result still undergoes normal strict GPU receipt parsing and independent
// CPU verification before a worker-signed PoUW v2 proof is returned.
func executeComputeGPUApprovedV066(
	ctx context.Context,
	task usefulwork.Task,
	jobID, chainID, genesisHash string,
	worker *wallet.Wallet,
	binaryPath, approvedSHA256 string,
) (approvedGPUExecutionV066, error) {
	var empty approvedGPUExecutionV066
	if ctx == nil || ctx.Err() != nil {
		return empty, fmt.Errorf("v0.66 approved GPU launch requires an active context")
	}
	if worker == nil {
		return empty, fmt.Errorf("v0.66 approved GPU launch requires a worker")
	}
	if err := validateGPUComputeTaskV062(task); err != nil {
		return empty, fmt.Errorf("v0.66 invalid GPU task: %w", err)
	}
	contract, err := usefulwork.NewQuantizedMLComputeTaskContractV066(
		task, jobID, chainID, genesisHash, approvedSHA256,
	)
	if err != nil {
		return empty, fmt.Errorf("v0.66 invalid task contract: %w", err)
	}

	request, err := json.Marshal(struct {
		Task usefulwork.Task `json:"task"`
	}{Task: task})
	if err != nil {
		return empty, fmt.Errorf("v0.66 encode GPU request: %w", err)
	}
	if len(request) > 512*1024 {
		return empty, fmt.Errorf("v0.66 GPU request exceeds 512 KiB")
	}

	stdout, err := runApprovedGPUWorkerV066(
		ctx, binaryPath, approvedSHA256, maxGPUExecutableBytesV066, request,
	)
	if err != nil {
		return empty, fmt.Errorf("v0.66 sealed GPU execution rejected: %w", err)
	}
	predictions, err := parseGPUReceiptV062(task, stdout)
	if err != nil {
		return empty, fmt.Errorf("v0.66 GPU receipt rejected: %w", err)
	}
	proof, err := usefulwork.BuildComputeMLProofV2FromResult(
		task, jobID, chainID, genesisHash, worker, predictions,
	)
	if err != nil {
		return empty, fmt.Errorf("v0.66 proof construction rejected: %w", err)
	}
	if err := usefulwork.VerifyQuantizedMLComputeTaskBindingV066(
		contract, proof, approvedSHA256,
	); err != nil {
		return empty, fmt.Errorf("v0.66 verified result does not match contract: %w", err)
	}
	return approvedGPUExecutionV066{Contract: contract, Proof: proof}, nil
}
