package usefulwork

import (
	"math"
	"testing"

	"prism/internal/wallet"
)

func TestMLInferenceBatch(
	t *testing.T,
) {

	task, err := NewMLInferenceBatchTask(
		2,
		3,
		2,
		[]uint64{
			3, 1, 0,
			0, 2, 4,
		},
		[]uint64{
			2, 0, 1,
			0, 3, 2,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err :=
		ComputeMLInferenceBatch(task)
	if err != nil {
		t.Fatal(err)
	}

	expected := []uint64{0, 1}

	if !equalUint64Slices(
		result,
		expected,
	) {
		t.Fatalf(
			"expected predictions %v, got %v",
			expected,
			result,
		)
	}

	if scoreForTask(task) != 12 {
		t.Fatalf(
			"expected score 12, got %d",
			scoreForTask(task),
		)
	}
}

func TestMLInferenceTieUsesLowestClass(
	t *testing.T,
) {

	task, err := NewMLInferenceBatchTask(
		1,
		2,
		2,
		[]uint64{1, 1},
		[]uint64{
			1, 0,
			0, 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err :=
		ComputeMLInferenceBatch(task)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 ||
		result[0] != 0 {

		t.Fatalf(
			"expected deterministic tie-break class 0, got %v",
			result,
		)
	}
}

func TestMLInferenceTamperedWeightsRejected(
	t *testing.T,
) {

	task, err := NewMLInferenceBatchTask(
		1,
		2,
		2,
		[]uint64{5, 7},
		[]uint64{
			2, 1,
			1, 2,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	task.ValuesB[0]++

	if err := ValidateTask(task); err == nil {
		t.Fatal(
			"expected tampered ML weights to be rejected",
		)
	}
}

func TestMLInferenceOverflow(
	t *testing.T,
) {

	task, err := NewMLInferenceBatchTask(
		1,
		1,
		1,
		[]uint64{math.MaxUint64},
		[]uint64{2},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err :=
		ComputeMLInferenceBatch(task); err == nil {

		t.Fatal(
			"expected ML inference multiplication overflow",
		)
	}
}

func TestMLInferenceExecuteAndVerifyProof(
	t *testing.T,
) {

	task, err := NewMLInferenceBatchTask(
		2,
		2,
		2,
		[]uint64{
			9, 1,
			1, 9,
		},
		[]uint64{
			3, 1,
			1, 3,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := Execute(
		task,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if proof.Result != 0 {
		t.Fatalf(
			"expected vector proof scalar result 0, got %d",
			proof.Result,
		)
	}

	expected := []uint64{0, 1}

	if !equalUint64Slices(
		proof.ResultValues,
		expected,
	) {
		t.Fatalf(
			"expected proof predictions %v, got %v",
			expected,
			proof.ResultValues,
		)
	}

	if err := VerifyProof(proof); err != nil {
		t.Fatalf(
			"valid ML inference proof rejected: %v",
			err,
		)
	}
}
