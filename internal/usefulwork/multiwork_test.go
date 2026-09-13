package usefulwork

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestSumSquaresBackwardCompatibility(
	t *testing.T,
) {

	task, err := NewSumSquaresTask(
		[]uint64{3, 4, 5},
	)
	if err != nil {
		t.Fatal(err)
	}

	const expectedInputHash = "251e3c94506e86df03eedb498e73490db6ee18a5218e27977f64c8399006937a"

	const expectedTaskID = "d0a681031c98ea49c6670e736d76cc93cf13679aa341f53e8cf8e41b5c2afd2f"

	if task.InputHash != expectedInputHash {
		t.Fatalf(
			"legacy input hash changed: %s",
			task.InputHash,
		)
	}

	if task.ID != expectedTaskID {
		t.Fatalf(
			"legacy task ID changed: %s",
			task.ID,
		)
	}

	result, err := Compute(task)
	if err != nil {
		t.Fatal(err)
	}

	if result != 50 {
		t.Fatalf(
			"expected 50, got %d",
			result,
		)
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(
		string(data),
		"values_b",
	) {
		t.Fatalf(
			"legacy sum_squares JSON contains values_b: %s",
			string(data),
		)
	}
}

func TestDotProduct(
	t *testing.T,
) {

	task, err := NewDotProductTask(
		[]uint64{2, 4, 6},
		[]uint64{3, 5, 7},
	)
	if err != nil {
		t.Fatal(err)
	}

	if task.Type != TaskTypeDotProduct {
		t.Fatalf(
			"unexpected task type: %s",
			task.Type,
		)
	}

	result, err := Compute(task)
	if err != nil {
		t.Fatal(err)
	}

	if result != 68 {
		t.Fatalf(
			"expected dot product 68, got %d",
			result,
		)
	}

	if scoreForTask(task) != 6 {
		t.Fatalf(
			"expected score 6, got %d",
			scoreForTask(task),
		)
	}
}

func TestDotProductRejectsDifferentLengths(
	t *testing.T,
) {

	_, err := NewDotProductTask(
		[]uint64{1, 2},
		[]uint64{3},
	)

	if err == nil {
		t.Fatal(
			"expected unequal vectors to be rejected",
		)
	}
}

func TestPrimeCount(
	t *testing.T,
) {

	task, err := NewPrimeCountTask(
		[]uint64{
			101,
			103,
			105,
			107,
			109,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Compute(task)
	if err != nil {
		t.Fatal(err)
	}

	if result != 4 {
		t.Fatalf(
			"expected 4 primes, got %d",
			result,
		)
	}

	if scoreForTask(task) != 5 {
		t.Fatalf(
			"expected score 5, got %d",
			scoreForTask(task),
		)
	}
}

func TestTamperedDotProductTaskRejected(
	t *testing.T,
) {

	task, err := NewDotProductTask(
		[]uint64{2, 4, 6},
		[]uint64{3, 5, 7},
	)
	if err != nil {
		t.Fatal(err)
	}

	task.ValuesB[0]++

	if err := ValidateTask(task); err == nil {
		t.Fatal(
			"expected tampered task to be rejected",
		)
	}
}

func TestSumSquaresOverflow(
	t *testing.T,
) {

	task, err := NewSumSquaresTask(
		[]uint64{math.MaxUint64},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Compute(task); err == nil {
		t.Fatal(
			"expected sum_squares overflow",
		)
	}
}

func TestDotProductMultiplicationOverflow(
	t *testing.T,
) {

	task, err := NewDotProductTask(
		[]uint64{math.MaxUint64},
		[]uint64{2},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Compute(task); err == nil {
		t.Fatal(
			"expected dot product multiplication overflow",
		)
	}
}

func TestDotProductAdditionOverflow(
	t *testing.T,
) {

	task, err := NewDotProductTask(
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

	if _, err := Compute(task); err == nil {
		t.Fatal(
			"expected dot product addition overflow",
		)
	}
}
