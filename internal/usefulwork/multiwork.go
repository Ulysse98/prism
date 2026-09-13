package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

func NewDotProductTask(
	valuesA []uint64,
	valuesB []uint64,
) (Task, error) {

	task := Task{
		Type:    TaskTypeDotProduct,
		Values:  cloneValues(valuesA),
		ValuesB: cloneValues(valuesB),
	}

	if err := validateDotProductTask(task); err != nil {
		return Task{}, err
	}

	inputHash, err :=
		calculateDotProductInputHash(
			task.Values,
			task.ValuesB,
		)
	if err != nil {
		return Task{}, err
	}

	task.InputHash = inputHash
	task.ID = CalculateTaskID(task)

	return task, nil
}

func NewPrimeCountTask(
	values []uint64,
) (Task, error) {

	task := Task{
		Type:   TaskTypePrimeCount,
		Values: cloneValues(values),
	}

	if err := validateSingleVectorTask(task); err != nil {
		return Task{}, err
	}

	inputHash, err := calculateInputHash(
		task.Values,
	)
	if err != nil {
		return Task{}, err
	}

	task.InputHash = inputHash
	task.ID = CalculateTaskID(task)

	return task, nil
}

func cloneValues(
	values []uint64,
) []uint64 {

	result := make(
		[]uint64,
		len(values),
	)

	copy(result, values)

	return result
}

func validateSingleVectorTask(
	task Task,
) error {

	if len(task.Values) == 0 {
		return fmt.Errorf(
			"useful work task cannot be empty",
		)
	}

	if len(task.ValuesB) != 0 {
		return fmt.Errorf(
			"task type %s does not accept a second input vector",
			task.Type,
		)
	}

	return nil
}

func validateDotProductTask(
	task Task,
) error {

	if len(task.Values) == 0 ||
		len(task.ValuesB) == 0 {

		return fmt.Errorf(
			"dot product vectors cannot be empty",
		)
	}

	if len(task.Values) !=
		len(task.ValuesB) {

		return fmt.Errorf(
			"dot product vectors must have equal length",
		)
	}

	return nil
}

func calculateDotProductInputHash(
	valuesA []uint64,
	valuesB []uint64,
) (string, error) {

	payload := struct {
		ValuesA []uint64 `json:"values_a"`
		ValuesB []uint64 `json:"values_b"`
	}{
		ValuesA: valuesA,
		ValuesB: valuesB,
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

func computeSumSquares(
	values []uint64,
) (uint64, error) {

	var result uint64

	for _, value := range values {
		if value != 0 &&
			value > math.MaxUint64/value {

			return 0, fmt.Errorf(
				"useful work multiplication overflow",
			)
		}

		square := value * value

		if result >
			math.MaxUint64-square {

			return 0, fmt.Errorf(
				"useful work addition overflow",
			)
		}

		result += square
	}

	return result, nil
}

func computeDotProduct(
	valuesA []uint64,
	valuesB []uint64,
) (uint64, error) {

	var result uint64

	for index := range valuesA {
		left := valuesA[index]
		right := valuesB[index]

		if right != 0 &&
			left > math.MaxUint64/right {

			return 0, fmt.Errorf(
				"useful work multiplication overflow",
			)
		}

		product := left * right

		if result >
			math.MaxUint64-product {

			return 0, fmt.Errorf(
				"useful work addition overflow",
			)
		}

		result += product
	}

	return result, nil
}

func computePrimeCount(
	values []uint64,
) uint64 {

	var count uint64

	for _, value := range values {
		if isPrime(value) {
			count++
		}
	}

	return count
}

func isPrime(
	value uint64,
) bool {

	if value < 2 {
		return false
	}

	if value == 2 {
		return true
	}

	if value%2 == 0 {
		return false
	}

	for divisor := uint64(3); divisor <= value/divisor; divisor += 2 {

		if value%divisor == 0 {
			return false
		}
	}

	return true
}
