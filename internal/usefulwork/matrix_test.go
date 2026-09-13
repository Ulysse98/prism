package usefulwork

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"prism/internal/wallet"
)

func TestMatrixMultiply(
	t *testing.T,
) {
	task, err :=
		NewMatrixMultiplyTask(
			2,
			3,
			2,
			[]uint64{
				1, 2, 3,
				4, 5, 6,
			},
			[]uint64{
				7, 8,
				9, 10,
				11, 12,
			},
		)
	if err != nil {
		t.Fatal(err)
	}

	result, err :=
		ComputeMatrix(task)
	if err != nil {
		t.Fatal(err)
	}

	expected := []uint64{
		58, 64,
		139, 154,
	}

	if !equalUint64Slices(
		result,
		expected,
	) {
		t.Fatalf(
			"expected %v, got %v",
			expected,
			result,
		)
	}

	if scoreForTask(task) != 12 {
		t.Fatalf(
			"expected 12 work units, got %d",
			scoreForTask(task),
		)
	}
}

func TestMatrixMultiplyProofRoundTrip(
	t *testing.T,
) {
	task, err :=
		NewMatrixMultiplyTask(
			2,
			3,
			2,
			[]uint64{
				1, 2, 3,
				4, 5, 6,
			},
			[]uint64{
				7, 8,
				9, 10,
				11, 12,
			},
		)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err :=
		Execute(
			task,
			worker,
		)
	if err != nil {
		t.Fatal(err)
	}

	expected := []uint64{
		58, 64,
		139, 154,
	}

	if proof.Result != 0 {
		t.Fatalf(
			"matrix proof contains scalar result: %d",
			proof.Result,
		)
	}

	if !equalUint64Slices(
		proof.ResultValues,
		expected,
	) {
		t.Fatalf(
			"unexpected matrix result: %v",
			proof.ResultValues,
		)
	}

	if proof.Score != 12 {
		t.Fatalf(
			"expected score 12, got %d",
			proof.Score,
		)
	}

	if err := VerifyProof(proof); err != nil {
		t.Fatalf(
			"valid matrix proof rejected: %v",
			err,
		)
	}
}

func TestMatrixProofTamperRejected(
	t *testing.T,
) {
	task, err :=
		NewMatrixMultiplyTask(
			1,
			2,
			1,
			[]uint64{2, 3},
			[]uint64{4, 5},
		)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err :=
		Execute(
			task,
			worker,
		)
	if err != nil {
		t.Fatal(err)
	}

	proof.ResultValues[0]++

	if err := VerifyProof(proof); err == nil {
		t.Fatal(
			"tampered matrix proof accepted",
		)
	}
}

func TestMatrixMultiplyRejectsInvalidShape(
	t *testing.T,
) {
	_, err :=
		NewMatrixMultiplyTask(
			2,
			3,
			2,
			[]uint64{
				1, 2, 3,
			},
			[]uint64{
				7, 8,
				9, 10,
				11, 12,
			},
		)

	if err == nil {
		t.Fatal(
			"expected invalid matrix shape rejection",
		)
	}
}

func TestMatrixTaskHashCommitsDimensions(
	t *testing.T,
) {
	task, err :=
		NewMatrixMultiplyTask(
			2,
			2,
			2,
			[]uint64{
				1, 2,
				3, 4,
			},
			[]uint64{
				5, 6,
				7, 8,
			},
		)
	if err != nil {
		t.Fatal(err)
	}

	task.RowsA++

	if err := ValidateTask(task); err == nil {
		t.Fatal(
			"tampered matrix dimensions accepted",
		)
	}
}

func TestMatrixMultiplyMultiplicationOverflow(
	t *testing.T,
) {
	task, err :=
		NewMatrixMultiplyTask(
			1,
			1,
			1,
			[]uint64{
				math.MaxUint64,
			},
			[]uint64{
				2,
			},
		)
	if err != nil {
		t.Fatal(err)
	}

	if _, err :=
		ComputeMatrix(task); err == nil {

		t.Fatal(
			"expected multiplication overflow",
		)
	}
}

func TestMatrixMultiplyAdditionOverflow(
	t *testing.T,
) {
	task, err :=
		NewMatrixMultiplyTask(
			1,
			2,
			1,
			[]uint64{
				math.MaxUint64,
				1,
			},
			[]uint64{
				1,
				1,
			},
		)
	if err != nil {
		t.Fatal(err)
	}

	if _, err :=
		ComputeMatrix(task); err == nil {

		t.Fatal(
			"expected addition overflow",
		)
	}
}

func TestLegacyTaskRejectsMatrixMetadata(
	t *testing.T,
) {
	task, err :=
		NewSumSquaresTask(
			[]uint64{3, 4, 5},
		)
	if err != nil {
		t.Fatal(err)
	}

	task.RowsA = 1

	if err := ValidateTask(task); err == nil {
		t.Fatal(
			"legacy task accepted matrix metadata",
		)
	}
}

func TestLegacyTaskJSONUnchanged(
	t *testing.T,
) {
	task, err :=
		NewSumSquaresTask(
			[]uint64{3, 4, 5},
		)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}

	serialized := string(data)

	for _, forbidden := range []string{
		"rows_a",
		"cols_a",
		"cols_b",
	} {

		if strings.Contains(
			serialized,
			forbidden,
		) {
			t.Fatalf(
				"legacy JSON unexpectedly contains %s: %s",
				forbidden,
				serialized,
			)
		}
	}
}
