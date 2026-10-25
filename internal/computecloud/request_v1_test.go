package computecloud

import (
	"fmt"
	"strings"
	"testing"

	"prism/internal/usefulwork"
)

const validJSON = `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`

func TestParseSubmitV1ValidAndCanonical(t *testing.T) {
	task, err := ParseSubmitV1([]byte(validJSON))
	if err != nil {
		t.Fatal(err)
	}
	if task.Type != usefulwork.TaskTypeMLInferenceQuantized ||
		task.RowsA != 1 || task.ColsA != 3 || task.ColsB != 3 {
		t.Fatalf("unexpected task contract: %+v", task)
	}
	if task.ID == "" || task.InputHash == "" ||
		task.ID != usefulwork.CalculateTaskID(task) {
		t.Fatal("derived Prism task commitments missing or inconsistent")
	}
	if err := ValidateTaskV1(task); err != nil {
		t.Fatal(err)
	}
	again, err := ParseSubmitV1([]byte(validJSON))
	if err != nil || again.ID != task.ID || again.InputHash != task.InputHash {
		t.Fatal("task commitments not deterministic")
	}
}

func TestParseSubmitV1RejectsUntrustedOrMalformedRequests(t *testing.T) {
	cases := map[string]string{
		"missing batch":        `{"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`,
		"wrong shape":          `{"batch_size":1,"inputs":[1,0],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`,
		"too many samples":     `{"batch_size":1025,"inputs":[],"weights":[],"biases":[]}`,
		"overflow value":       `{"batch_size":1,"inputs":[32768,0,0],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`,
		"negative overflow":    `{"batch_size":1,"inputs":[-32768,0,0],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`,
		"alias property":       strings.Replace(validJSON, `"batch_size":1`, `"BATCH_SIZE":1`, 1),
		"forbidden wallet":     strings.TrimSuffix(validJSON, "}") + `,"requester":"attacker"}`,
		"forbidden reward":     strings.TrimSuffix(validJSON, "}") + `,"reward":999999}`,
		"forbidden nonce":      strings.TrimSuffix(validJSON, "}") + `,"nonce":1}`,
		"forbidden task":       strings.TrimSuffix(validJSON, "}") + `,"task":{"type":"sum_squares"}}`,
		"forbidden input hash": strings.TrimSuffix(validJSON, "}") + `,"input_hash":"forged"}`,
		"duplicate key":        strings.TrimSuffix(validJSON, "}") + `,"batch_size":1}`,
		"trailing JSON":        validJSON + ` {}`,
		"null":                 `null`,
		"array root":           `[]`,
		"invalid field type":   strings.Replace(validJSON, `"batch_size":1`, `"batch_size":"1"`, 1),
		"empty body":           "",
		"oversized":            strings.Repeat(" ", MaxSubmitBodyBytesV1+1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if task, err := ParseSubmitV1([]byte(input)); err == nil {
				t.Fatalf("accepted dangerous input: %+v", task)
			}
		})
	}
}

func TestParseSubmitV1AllowsBoundedBatch(t *testing.T) {
	inputs := strings.TrimSuffix(strings.Repeat("0,", 3*int(MaxBatchSizeV1)), ",")
	payload := fmt.Sprintf(`{"batch_size":%d,"inputs":[%s],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`, MaxBatchSizeV1, inputs)
	task, err := ParseSubmitV1([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if task.RowsA != MaxBatchSizeV1 {
		t.Fatalf("unexpected batch %d", task.RowsA)
	}
}

func TestValidateTaskV1RejectsUnsupportedAndTampered(t *testing.T) {
	task, err := ParseSubmitV1([]byte(validJSON))
	if err != nil {
		t.Fatal(err)
	}
	task.Type = usefulwork.TaskTypeSumSquares
	if err := ValidateTaskV1(task); err == nil {
		t.Fatal("unsupported type accepted")
	}
	task, _ = ParseSubmitV1([]byte(validJSON))
	task.InputHash = strings.Repeat("0", 64)
	if err := ValidateTaskV1(task); err == nil {
		t.Fatal("tampered input hash accepted")
	}
	task, _ = ParseSubmitV1([]byte(validJSON))
	task.ColsA = 2
	if err := ValidateTaskV1(task); err == nil {
		t.Fatal("invalid dimensions accepted")
	}
}
