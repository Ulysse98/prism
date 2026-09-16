package main

import (
	"testing"

	"prism/internal/usefulwork"
)

func TestMineCatalogResponseHasSixWorkloads(
	t *testing.T,
) {
	response, err :=
		mineCatalogResponse(
			44,
			0,
		)
	if err != nil {
		t.Fatal(err)
	}

	if response.SourceChainHeight != 44 {
		t.Fatalf(
			"expected height 44, got %d",
			response.SourceChainHeight,
		)
	}

	if len(response.Tasks) != 6 {
		t.Fatalf(
			"expected 6 tasks, got %d",
			len(response.Tasks),
		)
	}

	expected := map[string]bool{
		usefulwork.TaskTypeSumSquares:       false,
		usefulwork.TaskTypeDotProduct:       false,
		usefulwork.TaskTypePrimeCount:       false,
		usefulwork.TaskTypeMatrixMultiply:   false,
		usefulwork.TaskTypeImageConvolution: false,
		usefulwork.TaskTypeMLInferenceBatch: false,
	}

	for _, entry := range response.Tasks {
		if entry.ID == "" {
			t.Fatalf(
				"empty task ID for %s",
				entry.Task,
			)
		}

		if entry.Difficulty == "" {
			t.Fatalf(
				"empty difficulty for %s",
				entry.Task,
			)
		}

		if entry.WorkUnits == 0 {
			t.Fatalf(
				"zero work units for %s",
				entry.Task,
			)
		}

		if _, ok := expected[entry.Task]; !ok {
			t.Fatalf(
				"unexpected workload: %s",
				entry.Task,
			)
		}

		expected[entry.Task] = true
	}

	for taskType, seen := range expected {
		if !seen {
			t.Fatalf(
				"missing workload: %s",
				taskType,
			)
		}
	}
}

func TestMineCatalogWorkUnits(
	t *testing.T,
) {
	response, err :=
		mineCatalogResponse(
			44,
			0,
		)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]uint64{
		usefulwork.TaskTypeSumSquares:       3,
		usefulwork.TaskTypeDotProduct:       6,
		usefulwork.TaskTypePrimeCount:       8,
		usefulwork.TaskTypeMatrixMultiply:   12,
		usefulwork.TaskTypeImageConvolution: 36,
		usefulwork.TaskTypeMLInferenceBatch: 27,
	}

	for _, entry := range response.Tasks {
		want, ok := expected[entry.Task]
		if !ok {
			t.Fatalf(
				"unexpected task: %s",
				entry.Task,
			)
		}

		if entry.WorkUnits != want {
			t.Fatalf(
				"%s: expected %d work units, got %d",
				entry.Task,
				want,
				entry.WorkUnits,
			)
		}
	}
}

func TestMineCatalogDeterministic(
	t *testing.T,
) {
	first, err :=
		mineCatalogResponse(
			44,
			0,
		)
	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		mineCatalogResponse(
			44,
			0,
		)
	if err != nil {
		t.Fatal(err)
	}

	for index := range first.Tasks {
		if first.Tasks[index] !=
			second.Tasks[index] {

			t.Fatalf(
				"catalog entry %d changed",
				index,
			)
		}
	}
}
