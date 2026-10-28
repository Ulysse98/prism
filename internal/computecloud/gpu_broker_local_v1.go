//go:build linux

package computecloud

// This operator-only integration binds an authenticated-by-context, pinned
// read-only Prism node observation to a single offline CUDA execution.
// It deliberately registers NO HTTP routes and NEVER submits a job or proof.

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
)

// CloudLocalBrokerGPUResultV1 is an offline, CPU-verified result, not a
// PoUW proof, invoice, escrow action, or authorization to dispatch jobs.
type CloudLocalBrokerGPUResultV1 struct {
	Mode                string   `json:"mode"`
	Workload            string   `json:"workload"`
	ChainID             string   `json:"chain_id"`
	NodeHeight          uint64   `json:"node_height"`
	TaskID              string   `json:"task_id"`
	InputHash           string   `json:"input_hash"`
	ResultValues        []uint64 `json:"result_values"`
	OutputHash          string   `json:"output_hash"`
	CPUVerified         bool     `json:"cpu_verified"`
	GPUAttestation      bool     `json:"gpu_attestation"`
	HTTPDispatchEnabled bool     `json:"http_dispatch_enabled"`
	EscrowEnabled       bool     `json:"escrow_enabled"`
	ProofsAvailable     bool     `json:"proofs_available"`
}

// RunLocalBrokerGPULabV1 requires operator confirmation of the exact canonical
// task ID, and read-only node status both before and after GPU execution.
// Node identity checks are pinning (not remote hardware attestation).
// Any validation failure returns no accepted result.
func RunLocalBrokerGPULabV1(ctx context.Context, data []byte,
	observer *CloudNodeObserverV1, approvedTaskID, gpuBinary, expectedSHA256 string,
) (CloudLocalBrokerGPUResultV1, error) {
	return runLocalBrokerGPULabV1(ctx, data, observer, approvedTaskID, gpuBinary, expectedSHA256, "")
}

// RunLocalBrokerGPULabWithGlobalLockV1 enforces the shared advisory lock for
// an operator-authorized offline GPU execution.
func RunLocalBrokerGPULabWithGlobalLockV1(ctx context.Context, data []byte,
	observer *CloudNodeObserverV1, approvedTaskID, gpuBinary, expectedSHA256, lockDir string,
) (CloudLocalBrokerGPUResultV1, error) {
	if lockDir == "" {
		return CloudLocalBrokerGPUResultV1{}, errors.New("GPU global lock directory required")
	}
	return runLocalBrokerGPULabV1(ctx, data, observer, approvedTaskID, gpuBinary, expectedSHA256, lockDir)
}

func runLocalBrokerGPULabV1(ctx context.Context, data []byte,
	observer *CloudNodeObserverV1, approvedTaskID, gpuBinary, expectedSHA256, lockDir string,
) (CloudLocalBrokerGPUResultV1, error) {
	var empty CloudLocalBrokerGPUResultV1
	if ctx == nil || observer == nil {
		return empty, errors.New("broker requires context and pinned node observer")
	}
	// Invalid or malformed workload must never probe the node or start CUDA.
	task, err := ParseSubmitV1(data)
	if err != nil {
		return empty, err
	}
	approvedBytes, err := hex.DecodeString(approvedTaskID)
	if err != nil || len(approvedBytes) != 32 || hex.EncodeToString(approvedBytes) != approvedTaskID ||
		subtle.ConstantTimeCompare([]byte(task.ID), []byte(approvedTaskID)) != 1 {
		return empty, errors.New("operator task-ID confirmation does not match canonical workload")
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	initial, err := observer.ReadStatus(ctx)
	if err != nil {
		return empty, fmt.Errorf("broker preflight node refused: %w", err)
	}
	if !initial.ChainValid || initial.ComputeEnabled || initial.ProofsAvailable {
		return empty, errors.New("broker node capabilities are not read-only")
	}
	var result CloudGPUResultV1
	if lockDir != "" {
		result, err = RunLocalGPULabWithGlobalLockV1(ctx, data, gpuBinary, expectedSHA256, lockDir)
	} else {
		result, err = RunLocalGPULabV1(ctx, data, gpuBinary, expectedSHA256)
	}
	if err != nil {
		return empty, fmt.Errorf("broker GPU execution refused: %w", err)
	}
	if !result.CPUVerified || result.GPUAttestation || result.DispatchEnabled || result.ProofsAvailable ||
		result.TaskID != task.ID || result.InputHash != task.InputHash {
		return empty, errors.New("broker offline GPU result is not safe")
	}
	// Fail closed if the observed node changes during this offline lab.
	// This is NOT a blockchain proof that no writes occurred: it detects
	// height/identity differences visible through the status endpoint.
	after, err := observer.ReadStatus(ctx)
	if err != nil {
		return empty, fmt.Errorf("broker postflight node refused: %w", err)
	}
	if after.ChainID != initial.ChainID || after.Height != initial.Height ||
		!after.ChainValid || after.ComputeEnabled || after.ProofsAvailable {
		return empty, errors.New("node changed during offline GPU lab")
	}
	return CloudLocalBrokerGPUResultV1{
		Mode: "operator_local_gpu_lab", Workload: task.Type,
		ChainID: initial.ChainID, NodeHeight: initial.Height,
		TaskID: result.TaskID, InputHash: result.InputHash,
		ResultValues: result.ResultValues, OutputHash: result.OutputHash,
		CPUVerified: true, GPUAttestation: false,
		HTTPDispatchEnabled: false, EscrowEnabled: false, ProofsAvailable: false,
	}, nil
}
