package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

const maxMLInferenceWorkUnits uint64 = 1 << 20

// NewMLInferenceBatchTask creates a deterministic quantized linear-classifier
// inference task.
//
// Values stores the input batch in row-major order:
//
//	batchSize x features
//
// ValuesB stores model weights in row-major order:
//
//	classes x features
//
// The output contains one class index per sample. Ties select the lowest class
// index so every Prism node reaches exactly the same result.
func NewMLInferenceBatchTask(
	batchSize uint64,
	features uint64,
	classes uint64,
	inputs []uint64,
	weights []uint64,
) (Task, error) {

	task := Task{
		Type:    TaskTypeMLInferenceBatch,
		Values:  cloneValues(inputs),
		ValuesB: cloneValues(weights),
		RowsA:   batchSize,
		ColsA:   features,
		ColsB:   classes,
	}

	if err := validateMLInferenceBatchTask(task); err != nil {
		return Task{}, err
	}

	inputHash, err :=
		calculateMLInferenceInputHash(task)
	if err != nil {
		return Task{}, err
	}

	task.InputHash = inputHash
	task.ID = CalculateTaskID(task)

	return task, nil
}

func validateMLInferenceBatchTask(
	task Task,
) error {

	if task.RowsA == 0 ||
		task.ColsA == 0 ||
		task.ColsB == 0 {

		return fmt.Errorf(
			"ML inference dimensions must be non-zero",
		)
	}

	expectedInputs, err :=
		checkedMulUint64(
			task.RowsA,
			task.ColsA,
		)
	if err != nil {
		return err
	}

	expectedWeights, err :=
		checkedMulUint64(
			task.ColsB,
			task.ColsA,
		)
	if err != nil {
		return err
	}

	if expectedInputs > maxMatrixElements ||
		expectedWeights > maxMatrixElements ||
		task.RowsA > maxMatrixElements {

		return fmt.Errorf(
			"ML inference task exceeds maximum size",
		)
	}

	if uint64(len(task.Values)) != expectedInputs {
		return fmt.Errorf(
			"ML inference expects %d input values, got %d",
			expectedInputs,
			len(task.Values),
		)
	}

	if uint64(len(task.ValuesB)) != expectedWeights {
		return fmt.Errorf(
			"ML inference expects %d weight values, got %d",
			expectedWeights,
			len(task.ValuesB),
		)
	}

	workUnits, err :=
		mlInferenceWorkUnitsChecked(task)
	if err != nil {
		return err
	}

	if workUnits > maxMLInferenceWorkUnits {
		return fmt.Errorf(
			"ML inference task exceeds maximum work units: %d > %d",
			workUnits,
			maxMLInferenceWorkUnits,
		)
	}

	return nil
}

func calculateMLInferenceInputHash(
	task Task,
) (string, error) {

	payload := struct {
		BatchSize uint64   `json:"batch_size"`
		Features  uint64   `json:"features"`
		Classes   uint64   `json:"classes"`
		Inputs    []uint64 `json:"inputs"`
		Weights   []uint64 `json:"weights"`
	}{
		BatchSize: task.RowsA,
		Features:  task.ColsA,
		Classes:   task.ColsB,
		Inputs:    task.Values,
		Weights:   task.ValuesB,
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

func ComputeMLInferenceBatch(
	task Task,
) ([]uint64, error) {

	if err := ValidateTask(task); err != nil {
		return nil, err
	}

	if task.Type != TaskTypeMLInferenceBatch {
		return nil, fmt.Errorf(
			"task type %s is not an ML inference batch",
			task.Type,
		)
	}

	return computeMLInferenceBatch(task)
}

func computeMLInferenceBatch(
	task Task,
) ([]uint64, error) {

	predictions := make(
		[]uint64,
		int(task.RowsA),
	)

	for sample := uint64(0); sample < task.RowsA; sample++ {
		var bestClass uint64
		var bestScore uint64

		for class := uint64(0); class < task.ColsB; class++ {
			var score uint64

			for feature := uint64(0); feature < task.ColsA; feature++ {
				inputIndex :=
					sample*task.ColsA +
						feature

				weightIndex :=
					class*task.ColsA +
						feature

				input :=
					task.Values[inputIndex]

				weight :=
					task.ValuesB[weightIndex]

				if weight != 0 &&
					input > math.MaxUint64/weight {

					return nil, fmt.Errorf(
						"ML inference multiplication overflow",
					)
				}

				product := input * weight

				if score >
					math.MaxUint64-product {

					return nil, fmt.Errorf(
						"ML inference addition overflow",
					)
				}

				score += product
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

func mlInferenceWorkUnitsChecked(
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

func mlInferenceWorkUnits(
	task Task,
) uint64 {

	score, err :=
		mlInferenceWorkUnitsChecked(task)

	if err != nil {
		return 0
	}

	return score
}
