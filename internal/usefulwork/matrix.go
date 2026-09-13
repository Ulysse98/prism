package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

const maxMatrixElements uint64 = 4096

func NewMatrixMultiplyTask(
	rowsA uint64,
	colsA uint64,
	colsB uint64,
	valuesA []uint64,
	valuesB []uint64,
) (Task, error) {

	task := Task{
		Type:    TaskTypeMatrixMultiply,
		Values:  cloneValues(valuesA),
		ValuesB: cloneValues(valuesB),
		RowsA:   rowsA,
		ColsA:   colsA,
		ColsB:   colsB,
	}

	if err :=
		validateMatrixMultiplyTask(task); err != nil {

		return Task{}, err
	}

	inputHash, err :=
		calculateMatrixMultiplyInputHash(
			task,
		)
	if err != nil {
		return Task{}, err
	}

	task.InputHash = inputHash
	task.ID = CalculateTaskID(task)

	return task, nil
}

func validateMatrixMultiplyTask(
	task Task,
) error {

	if task.RowsA == 0 ||
		task.ColsA == 0 ||
		task.ColsB == 0 {

		return fmt.Errorf(
			"matrix dimensions must be non-zero",
		)
	}

	expectedA, err :=
		checkedMulUint64(
			task.RowsA,
			task.ColsA,
		)
	if err != nil {
		return err
	}

	expectedB, err :=
		checkedMulUint64(
			task.ColsA,
			task.ColsB,
		)
	if err != nil {
		return err
	}

	expectedOutput, err :=
		checkedMulUint64(
			task.RowsA,
			task.ColsB,
		)
	if err != nil {
		return err
	}

	if expectedA > maxMatrixElements ||
		expectedB > maxMatrixElements ||
		expectedOutput > maxMatrixElements {

		return fmt.Errorf(
			"matrix task exceeds maximum size",
		)
	}

	if uint64(len(task.Values)) !=
		expectedA {

		return fmt.Errorf(
			"matrix A expects %d values, got %d",
			expectedA,
			len(task.Values),
		)
	}

	if uint64(len(task.ValuesB)) !=
		expectedB {

		return fmt.Errorf(
			"matrix B expects %d values, got %d",
			expectedB,
			len(task.ValuesB),
		)
	}

	_, err = matrixWorkUnitsChecked(task)
	if err != nil {
		return err
	}

	return nil
}

func calculateMatrixMultiplyInputHash(
	task Task,
) (string, error) {

	payload := struct {
		RowsA   uint64   `json:"rows_a"`
		ColsA   uint64   `json:"cols_a"`
		ColsB   uint64   `json:"cols_b"`
		ValuesA []uint64 `json:"values_a"`
		ValuesB []uint64 `json:"values_b"`
	}{
		RowsA:   task.RowsA,
		ColsA:   task.ColsA,
		ColsB:   task.ColsB,
		ValuesA: task.Values,
		ValuesB: task.ValuesB,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(
		hash[:],
	), nil
}

func ComputeMatrix(
	task Task,
) ([]uint64, error) {

	if err := ValidateTask(task); err != nil {
		return nil, err
	}

	if task.Type !=
		TaskTypeMatrixMultiply {

		return nil, fmt.Errorf(
			"task type %s does not return a matrix",
			task.Type,
		)
	}

	return computeMatrixMultiply(task)
}

func computeMatrixMultiply(
	task Task,
) ([]uint64, error) {

	outputCount, err :=
		checkedMulUint64(
			task.RowsA,
			task.ColsB,
		)
	if err != nil {
		return nil, err
	}

	result := make(
		[]uint64,
		int(outputCount),
	)

	for row := uint64(0); row < task.RowsA; row++ {

		for col := uint64(0); col < task.ColsB; col++ {

			var cell uint64

			for inner := uint64(0); inner < task.ColsA; inner++ {

				aIndex :=
					row*task.ColsA +
						inner

				bIndex :=
					inner*task.ColsB +
						col

				left :=
					task.Values[aIndex]

				right :=
					task.ValuesB[bIndex]

				if right != 0 &&
					left >
						math.MaxUint64/right {

					return nil, fmt.Errorf(
						"useful work matrix multiplication overflow",
					)
				}

				product :=
					left * right

				if cell >
					math.MaxUint64-product {

					return nil, fmt.Errorf(
						"useful work matrix addition overflow",
					)
				}

				cell += product
			}

			resultIndex :=
				row*task.ColsB +
					col

			result[resultIndex] = cell
		}
	}

	return result, nil
}

func calculateValuesOutputHash(
	values []uint64,
) (string, error) {

	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(
		hash[:],
	), nil
}

func matrixWorkUnitsChecked(
	task Task,
) (uint64, error) {

	first, err :=
		checkedMulUint64(
			task.RowsA,
			task.ColsA,
		)
	if err != nil {
		return 0, err
	}

	return checkedMulUint64(
		first,
		task.ColsB,
	)
}

func matrixWorkUnits(
	task Task,
) uint64 {

	score, err :=
		matrixWorkUnitsChecked(task)

	if err != nil {
		return 0
	}

	return score
}

func checkedMulUint64(
	left uint64,
	right uint64,
) (uint64, error) {

	if right != 0 &&
		left > math.MaxUint64/right {

		return 0, fmt.Errorf(
			"useful work dimension overflow",
		)
	}

	return left * right, nil
}

func equalUint64Slices(
	left []uint64,
	right []uint64,
) bool {

	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}
