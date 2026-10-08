package usefulwork

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type quantumWorkerRequest struct {
	Circuit string `json:"circuit"`
	Qubits  uint64 `json:"qubits"`
	Shots   uint64 `json:"shots"`
	Target  string `json:"target"`
}

type quantumWorkerResponse struct {
	OK       bool              `json:"ok"`
	Backend  string            `json:"backend"`
	Workload string            `json:"workload"`
	Shots    uint64            `json:"shots"`
	Counts   map[string]uint64 `json:"counts"`
	Error    string            `json:"error,omitempty"`
}

func executeExternalQuantumSimulation(
	task Task,
) ([]uint64, error) {

	qubits, shots, err :=
		QuantumSimulationParameters(task)
	if err != nil {
		return nil, err
	}

	target := os.Getenv(
		"PRISM_QUANTUM_TARGET",
	)
	if target == "" {
		target = "nvidia"
	}

	request := quantumWorkerRequest{
		Circuit: QuantumCircuitBell,
		Qubits:  qubits,
		Shots:   shots,
		Target:  target,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf(
			"encode quantum worker request: %w",
			err,
		)
	}

	script := os.Getenv("PRISM_QUANTUM_SCRIPT")
	if script == "" {
		script =
			"/mnt/c/Users/ulyss/prism/scripts/prism_quantum_worker.py"
	}

	python := os.Getenv("PRISM_QUANTUM_PYTHON")
	if python == "" {
		python =
			"/home/ulyss/.venvs/prism-cudaq/bin/python"
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"wsl",
		python,
		script,
	)

	cmd.Stdin = bytes.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"quantum worker failed: %w: %s",
			err,
			stderr.String(),
		)
	}

	var response quantumWorkerResponse

	if err := json.Unmarshal(
		bytes.TrimSpace(stdout.Bytes()),
		&response,
	); err != nil {
		return nil, fmt.Errorf(
			"decode quantum worker response: %w: %s",
			err,
			stdout.String(),
		)
	}

	return validateQuantumWorkerResponse(response, target, shots)
}

func validateQuantumWorkerResponse(response quantumWorkerResponse, target string, shots uint64) ([]uint64, error) {
	if !response.OK {
		return nil, fmt.Errorf(
			"quantum worker error: %s",
			response.Error,
		)
	}

	if response.Shots != shots {
		return nil, fmt.Errorf(
			"quantum worker returned %d shots, expected %d",
			response.Shots,
			shots,
		)
	}

	if response.Backend != "cudaq:"+target {
		return nil, fmt.Errorf(
			"unexpected quantum worker backend: %s",
			response.Backend,
		)
	}

	if response.Workload != QuantumCircuitBell {
		return nil, fmt.Errorf(
			"unexpected quantum worker workload: %s",
			response.Workload,
		)
	}

	if len(response.Counts) != 4 {
		return nil, fmt.Errorf(
			"bell worker requires exactly 4 states",
		)
	}

	for _, state := range []string{"00", "01", "10", "11"} {
		if _, ok := response.Counts[state]; !ok {
			return nil, fmt.Errorf(
				"missing quantum state: %s",
				state,
			)
		}
	}

	counts := []uint64{
		response.Counts["00"],
		response.Counts["01"],
		response.Counts["10"],
		response.Counts["11"],
	}

	if err := VerifyBellCounts(
		counts,
		shots,
	); err != nil {
		return nil, err
	}

	return counts, nil
}
