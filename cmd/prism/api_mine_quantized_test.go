package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"prism/internal/usefulwork"
)

func TestQuantizedMLMineJobJSONRoundTrip(t *testing.T) {
	option, err := mineTaskForHeightAndType(
		27,
		usefulwork.TaskTypeMLInferenceQuantized,
	)
	if err != nil {
		t.Fatal(err)
	}

	task := option.Task
	if len(task.Values) != 0 || len(task.ValuesB) != 0 {
		t.Fatal("quantized catalog task unexpectedly uses unsigned values")
	}

	hasNegative := false
	for _, values := range [][]int64{
		task.SignedValues,
		task.SignedValuesB,
		task.Biases,
	} {
		for _, value := range values {
			if value < 0 {
				hasNegative = true
			}
		}
	}
	if !hasNegative {
		t.Fatal("quantized catalog task must contain negative values")
	}

	workUnits, err := usefulwork.WorkUnits(task)
	if err != nil {
		t.Fatal(err)
	}
	if workUnits != 27 {
		t.Fatalf("expected 27 work units, got %d", workUnits)
	}

	encoded, err := json.Marshal(apiMineJobFromTask(task))
	if err != nil {
		t.Fatal(err)
	}

	for _, field := range [][]byte{
		[]byte(`"signedValues"`),
		[]byte(`"signedValuesB"`),
		[]byte(`"biases"`),
	} {
		if !bytes.Contains(encoded, field) {
			t.Fatalf("mine job JSON is missing field %s: %s", field, encoded)
		}
	}

	var decoded apiMineJob
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	roundTrip := decoded.usefulWorkTask()
	if err := usefulwork.ValidateTask(roundTrip); err != nil {
		t.Fatalf("round-tripped task is invalid: %v", err)
	}
	if roundTrip.ID != task.ID {
		t.Fatalf("expected task ID %s, got %s", task.ID, roundTrip.ID)
	}

	want, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}
	got, err := usefulwork.ComputeMLInferenceQuantized(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected predictions %v, got %v", want, got)
	}
}
