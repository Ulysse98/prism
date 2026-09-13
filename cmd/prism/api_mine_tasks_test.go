package main

import (
	"testing"

	"prism/internal/usefulwork"
)

func TestMineTaskCatalogHasFourWorkloads(
	t *testing.T,
) {

	options, err :=
		mineTaskCatalogForHeight(18)
	if err != nil {
		t.Fatal(err)
	}

	if len(options) != 4 {
		t.Fatalf(
			"expected 4 workloads, got %d",
			len(options),
		)
	}

	seenTypes := map[string]bool{}
	seenIDs := map[string]bool{}

	for _, option := range options {
		task := option.Task

		if err := usefulwork.ValidateTask(
			task,
		); err != nil {
			t.Fatalf(
				"invalid task %s: %v",
				task.Type,
				err,
			)
		}

		if seenTypes[task.Type] {
			t.Fatalf(
				"duplicate task type: %s",
				task.Type,
			)
		}

		if seenIDs[task.ID] {
			t.Fatalf(
				"duplicate task ID: %s",
				task.ID,
			)
		}

		seenTypes[task.Type] = true
		seenIDs[task.ID] = true
	}

	for _, expected := range []string{
		usefulwork.TaskTypeSumSquares,
		usefulwork.TaskTypeDotProduct,
		usefulwork.TaskTypePrimeCount,
		usefulwork.TaskTypeMatrixMultiply,
	} {
		if !seenTypes[expected] {
			t.Fatalf(
				"missing workload: %s",
				expected,
			)
		}
	}
}

func TestMineTaskDefaultRemainsSumSquares(
	t *testing.T,
) {

	option, err :=
		mineTaskForHeightAndType(
			42,
			"",
		)
	if err != nil {
		t.Fatal(err)
	}

	if option.Task.Type !=
		usefulwork.TaskTypeSumSquares {

		t.Fatalf(
			"expected default %s, got %s",
			usefulwork.TaskTypeSumSquares,
			option.Task.Type,
		)
	}
}

func TestMineTaskSelectionByType(
	t *testing.T,
) {

	taskTypes := []string{
		usefulwork.TaskTypeSumSquares,
		usefulwork.TaskTypeDotProduct,
		usefulwork.TaskTypePrimeCount,
	}

	for _, taskType := range taskTypes {

		option, err :=
			mineTaskForHeightAndType(
				27,
				taskType,
			)
		if err != nil {
			t.Fatal(err)
		}

		if option.Task.Type != taskType {
			t.Fatalf(
				"expected %s, got %s",
				taskType,
				option.Task.Type,
			)
		}
	}
}

func TestMineTaskSelectionRejectsUnknown(
	t *testing.T,
) {

	_, err :=
		mineTaskForHeightAndType(
			10,
			"fake_work",
		)

	if err == nil {
		t.Fatal(
			"expected unsupported task rejection",
		)
	}
}

func TestMineTaskLookupByJobID(
	t *testing.T,
) {

	const height uint64 = 73

	options, err :=
		mineTaskCatalogForHeight(height)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range options {

		actual, err :=
			mineTaskForJobID(
				height,
				expected.Task.ID,
			)
		if err != nil {
			t.Fatal(err)
		}

		if actual.Task.ID !=
			expected.Task.ID {

			t.Fatalf(
				"expected job %s, got %s",
				expected.Task.ID,
				actual.Task.ID,
			)
		}

		if actual.Task.Type !=
			expected.Task.Type {

			t.Fatalf(
				"expected type %s, got %s",
				expected.Task.Type,
				actual.Task.Type,
			)
		}
	}
}

func TestMineTaskLookupRejectsUnknownJob(
	t *testing.T,
) {

	_, err :=
		mineTaskForJobID(
			15,
			"not-a-real-job",
		)

	if err == nil {
		t.Fatal(
			"expected invalid job ID rejection",
		)
	}
}

func TestMineTaskCatalogDeterministic(
	t *testing.T,
) {

	first, err :=
		mineTaskCatalogForHeight(55)
	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		mineTaskCatalogForHeight(55)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != len(second) {
		t.Fatal(
			"catalog length changed",
		)
	}

	for index := range first {
		if first[index].Task.ID !=
			second[index].Task.ID {

			t.Fatalf(
				"task %d is not deterministic",
				index,
			)
		}
	}
}
