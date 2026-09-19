package usefulwork

import (
	"math"
	"testing"

	"prism/internal/wallet"
)

func TestMLInferenceQuantizedSignedValuesAndBiases(t *testing.T) {
	task, err := NewMLInferenceQuantizedTask(
		2,
		2,
		2,
		[]int64{
			2, -3,
			-1, 4,
		},
		[]int64{
			2, 1,
			-1, -2,
		},
		[]int64{1, -1},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}

	expected := []uint64{1, 0}
	if !equalUint64Slices(result, expected) {
		t.Fatalf("expected predictions %v, got %v", expected, result)
	}

	workUnits, err := WorkUnits(task)
	if err != nil {
		t.Fatal(err)
	}
	if workUnits != 8 {
		t.Fatalf("expected 8 work units, got %d", workUnits)
	}
}

func TestMLInferenceQuantizedTieUsesLowestClass(t *testing.T) {
	task, err := NewMLInferenceQuantizedTask(
		1,
		2,
		2,
		[]int64{-2, 3},
		[]int64{
			1, 1,
			1, 1,
		},
		[]int64{4, 4},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 || result[0] != 0 {
		t.Fatalf("expected deterministic tie-break class 0, got %v", result)
	}
}

func TestMLInferenceQuantizedTamperedBiasRejected(t *testing.T) {
	task, err := NewMLInferenceQuantizedTask(
		1,
		1,
		2,
		[]int64{-3},
		[]int64{2, -2},
		[]int64{1, -1},
	)
	if err != nil {
		t.Fatal(err)
	}

	task.Biases[0]++
	if err := ValidateTask(task); err == nil {
		t.Fatal("expected tampered quantized ML bias to be rejected")
	}
}

func TestMLInferenceQuantizedMultiplicationOverflow(t *testing.T) {
	task, err := NewMLInferenceQuantizedTask(
		1,
		1,
		1,
		[]int64{math.MinInt64},
		[]int64{-1},
		[]int64{0},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ComputeMLInferenceQuantized(task); err == nil {
		t.Fatal("expected quantized ML multiplication overflow")
	}
}

func TestMLInferenceQuantizedAdditionOverflow(t *testing.T) {
	task, err := NewMLInferenceQuantizedTask(
		1,
		1,
		1,
		[]int64{math.MaxInt64},
		[]int64{1},
		[]int64{1},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ComputeMLInferenceQuantized(task); err == nil {
		t.Fatal("expected quantized ML addition overflow")
	}
}

func TestMLInferenceQuantizedExecuteAndVerifyProof(t *testing.T) {
	task, err := NewMLInferenceQuantizedTask(
		2,
		2,
		2,
		[]int64{
			2, -3,
			-1, 4,
		},
		[]int64{
			2, 1,
			-1, -2,
		},
		[]int64{1, -1},
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := Execute(task, worker)
	if err != nil {
		t.Fatal(err)
	}

	if proof.Result != 0 {
		t.Fatalf("expected vector proof scalar result 0, got %d", proof.Result)
	}

	expected := []uint64{1, 0}
	if !equalUint64Slices(proof.ResultValues, expected) {
		t.Fatalf(
			"expected proof predictions %v, got %v",
			expected,
			proof.ResultValues,
		)
	}

	if err := VerifyProof(proof); err != nil {
		t.Fatalf("valid quantized ML proof rejected: %v", err)
	}

	proof.ResultValues[0] = 0
	if err := VerifyProof(proof); err == nil {
		t.Fatal("expected tampered quantized ML proof to be rejected")
	}
}
