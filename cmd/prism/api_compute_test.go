package main

import (
	"strings"
	"testing"

	"prism/internal/usefulwork"
)

func TestAPIComputeCreateBuildsDotProductTask(t *testing.T) {
	payload := apiComputeCreateJobRequest{
		Type:    usefulwork.TaskTypeDotProduct,
		Values:  []uint64{10, 20, 30},
		ValuesB: []uint64{4, 5, 6},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if task.Type != usefulwork.TaskTypeDotProduct {
		t.Fatalf("unexpected task type: %s", task.Type)
	}

	if task.ID == "" || task.InputHash == "" {
		t.Fatal("expected canonical task ID and input hash")
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf("generated task is invalid: %v", err)
	}
}

func TestAPIComputeCreateBuildsMatrixTask(t *testing.T) {
	payload := apiComputeCreateJobRequest{
		Type:    usefulwork.TaskTypeMatrixMultiply,
		RowsA:   2,
		ColsA:   2,
		ColsB:   2,
		Values:  []uint64{1, 2, 3, 4},
		ValuesB: []uint64{5, 6, 7, 8},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf("generated matrix task is invalid: %v", err)
	}
}

func TestAPIComputeCreateBuildsConvolutionTask(t *testing.T) {
	payload := apiComputeCreateJobRequest{
		Type:       usefulwork.TaskTypeImageConvolution,
		Rows:       3,
		Cols:       3,
		KernelSize: 2,
		Values: []uint64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
		ValuesB: []uint64{
			1, 0,
			0, 1,
		},
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if err := usefulwork.ValidateTask(task); err != nil {
		t.Fatalf("generated convolution task is invalid: %v", err)
	}
}

func TestAPIComputeCreatePreservesLegacyTask(t *testing.T) {
	legacy, err := usefulwork.NewPrimeCountTask(
		[]uint64{2, 3, 4, 5, 11},
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := apiComputeCreateJobRequest{
		Task: &legacy,
	}

	task, err := payload.buildTask()
	if err != nil {
		t.Fatal(err)
	}

	if task.ID != legacy.ID {
		t.Fatalf(
			"legacy task ID changed: got %s want %s",
			task.ID,
			legacy.ID,
		)
	}
}

func TestAPIComputeCreateRejectsMixedTaskFormats(t *testing.T) {
	legacy, err := usefulwork.NewSumSquaresTask(
		[]uint64{1, 2, 3},
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := apiComputeCreateJobRequest{
		Task:   &legacy,
		Type:   usefulwork.TaskTypeSumSquares,
		Values: []uint64{1, 2, 3},
	}

	_, err = payload.buildTask()
	if err == nil {
		t.Fatal("expected mixed task format to be rejected")
	}

	if !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("unexpected error: %v", err)
	}
}
