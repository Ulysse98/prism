package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

// NewMLInferenceQuantizedTask creates a deterministic signed quantized
// linear-classifier inference task. It is versioned separately from the
// unsigned v0.36 workload so existing tasks and proofs remain compatible.
func NewMLInferenceQuantizedTask(
	batchSize uint64,
	features uint64,
	classes uint64,
	inputs []int64,
	weights []int64,
	biases []int64,
) (Task, error) {
	task := Task{
		Type:          TaskTypeMLInferenceQuantized,
		SignedValues:  cloneInt64Values(inputs),
		SignedValuesB: cloneInt64Values(weights),
		Biases:        cloneInt64Values(biases),
		RowsA:         batchSize,
		ColsA:         features,
		ColsB:         classes,
	}

	if err := validateMLInferenceQuantizedTask(task); err != nil {
		return Task{}, err
	}

	inputHash, err := calculateMLInferenceQuantizedInputHash(task)
	if err != nil {
		return Task{}, err
	}

	task.InputHash = inputHash
	task.ID = CalculateTaskID(task)

	return task, nil
}

func cloneInt64Values(values []int64) []int64 {
	cloned := make([]int64, len(values))
	copy(cloned, values)
	return cloned
}

func validateMLInferenceQuantizedTask(task Task) error {
	if task.RowsA == 0 || task.ColsA == 0 || task.ColsB == 0 {
		return fmt.Errorf("quantized ML inference dimensions must be non-zero")
	}

	if len(task.Values) != 0 || len(task.ValuesB) != 0 {
		return fmt.Errorf("quantized ML inference must use signed values")
	}

	expectedInputs, err := checkedMulUint64(task.RowsA, task.ColsA)
	if err != nil {
		return err
	}

	expectedWeights, err := checkedMulUint64(task.ColsB, task.ColsA)
	if err != nil {
		return err
	}

	if expectedInputs > maxMatrixElements ||
		expectedWeights > maxMatrixElements ||
		task.RowsA > maxMatrixElements ||
		task.ColsB > maxMatrixElements {
		return fmt.Errorf("quantized ML inference task exceeds maximum size")
	}

	if uint64(len(task.SignedValues)) != expectedInputs {
		return fmt.Errorf(
			"quantized ML inference expects %d input values, got %d",
			expectedInputs,
			len(task.SignedValues),
		)
	}

	if uint64(len(task.SignedValuesB)) != expectedWeights {
		return fmt.Errorf(
			"quantized ML inference expects %d weight values, got %d",
			expectedWeights,
			len(task.SignedValuesB),
		)
	}

	if uint64(len(task.Biases)) != task.ColsB {
		return fmt.Errorf(
			"quantized ML inference expects %d biases, got %d",
			task.ColsB,
			len(task.Biases),
		)
	}

	workUnits, err := mlInferenceWorkUnitsChecked(task)
	if err != nil {
		return err
	}

	if workUnits > maxMLInferenceWorkUnits {
		return fmt.Errorf(
			"quantized ML inference task exceeds maximum work units: %d > %d",
			workUnits,
			maxMLInferenceWorkUnits,
		)
	}

	return nil
}

func calculateMLInferenceQuantizedInputHash(task Task) (string, error) {
	payload := struct {
		BatchSize uint64  `json:"batch_size"`
		Features  uint64  `json:"features"`
		Classes   uint64  `json:"classes"`
		Inputs    []int64 `json:"inputs"`
		Weights   []int64 `json:"weights"`
		Biases    []int64 `json:"biases"`
	}{
		BatchSize: task.RowsA,
		Features:  task.ColsA,
		Classes:   task.ColsB,
		Inputs:    task.SignedValues,
		Weights:   task.SignedValuesB,
		Biases:    task.Biases,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ComputeMLInferenceQuantized evaluates signed int64 inputs, weights and
// per-class biases. Equal scores select the lowest class index.
func ComputeMLInferenceQuantized(task Task) ([]uint64, error) {
	if err := ValidateTask(task); err != nil {
		return nil, err
	}

	if task.Type != TaskTypeMLInferenceQuantized {
		return nil, fmt.Errorf(
			"task type %s is not a quantized ML inference batch",
			task.Type,
		)
	}

	predictions := make([]uint64, int(task.RowsA))

	for sample := uint64(0); sample < task.RowsA; sample++ {
		var bestClass uint64
		var bestScore int64

		for class := uint64(0); class < task.ColsB; class++ {
			score := task.Biases[class]

			for feature := uint64(0); feature < task.ColsA; feature++ {
				inputIndex := sample*task.ColsA + feature
				weightIndex := class*task.ColsA + feature

				product, err := checkedMulInt64(
					task.SignedValues[inputIndex],
					task.SignedValuesB[weightIndex],
				)
				if err != nil {
					return nil, fmt.Errorf(
						"quantized ML inference multiplication overflow",
					)
				}

				score, err = checkedAddInt64(score, product)
				if err != nil {
					return nil, fmt.Errorf(
						"quantized ML inference addition overflow",
					)
				}
			}

			if class == 0 || score > bestScore {
				bestClass = class
				bestScore = score
			}
		}

		predictions[sample] = bestClass
	}

	return predictions, nil
}

func checkedMulInt64(a int64, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}

	if (a == math.MinInt64 && b == -1) ||
		(b == math.MinInt64 && a == -1) {
		return 0, fmt.Errorf("int64 multiplication overflow")
	}

	result := a * b
	if result/b != a {
		return 0, fmt.Errorf("int64 multiplication overflow")
	}

	return result, nil
}

func checkedAddInt64(a int64, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, fmt.Errorf("int64 addition overflow")
	}

	if b < 0 && a < math.MinInt64-b {
		return 0, fmt.Errorf("int64 addition overflow")
	}

	return a + b, nil
}
