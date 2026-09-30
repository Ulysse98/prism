package main

import (
	"testing"

	"prism/internal/usefulwork"
)

func TestAPIComputeCreateBuildsQuantumSimulationTask(
	t *testing.T,
) {
	payload := apiComputeCreateJobRequest{
		Type: usefulwork.TaskTypeQuantumSimulation,
		Values: []uint64{
			2,
			4096,
		},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatalf(
			"build quantum simulation task: %v",
			err,
		)
	}

	if task.Type != usefulwork.TaskTypeQuantumSimulation {
		t.Fatalf(
			"expected task type %q, got %q",
			usefulwork.TaskTypeQuantumSimulation,
			task.Type,
		)
	}

	if len(task.Values) != 2 {
		t.Fatalf(
			"expected 2 quantum values, got %d",
			len(task.Values),
		)
	}

	if task.Values[0] != 2 {
		t.Fatalf(
			"expected 2 qubits, got %d",
			task.Values[0],
		)
	}

	if task.Values[1] != 4096 {
		t.Fatalf(
			"expected 4096 shots, got %d",
			task.Values[1],
		)
	}

	if task.InputHash == "" {
		t.Fatal(
			"expected quantum simulation input hash",
		)
	}

	if task.ID == "" {
		t.Fatal(
			"expected quantum simulation task ID",
		)
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf(
			"quantum simulation API task rejected: %v",
			err,
		)
	}

	workUnits, err := usefulwork.WorkUnits(task)
	if err != nil {
		t.Fatalf(
			"quantum simulation work units: %v",
			err,
		)
	}

	if workUnits != 4096 {
		t.Fatalf(
			"expected 4096 work units, got %d",
			workUnits,
		)
	}
}

func TestAPIComputeCreateRejectsInvalidQuantumValues(
	t *testing.T,
) {
	payload := apiComputeCreateJobRequest{
		Type: usefulwork.TaskTypeQuantumSimulation,
		Values: []uint64{
			2,
		},
	}

	if _, err := payload.buildTask(); err == nil {
		t.Fatal(
			"expected invalid quantum values to be rejected",
		)
	}
}
