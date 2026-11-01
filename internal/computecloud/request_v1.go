// Package computecloud defines the SaaS-facing input contract for Prism Cloud.
// It does not register HTTP routes, create marketplace jobs, or access wallets.
package computecloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"prism/internal/usefulwork"
)

const (
	// MaxSubmitBodyBytesV1 is a strict pre-parse upper bound for SaaS input.
	MaxSubmitBodyBytesV1 = 64 * 1024
	// MaxBatchSizeV1 is deliberately below the worker's actual upper bound.
	MaxBatchSizeV1 uint64 = 1024
	// MaxAbsValueV1 is a conservative MVP bound against signed arithmetic overflow.
	MaxAbsValueV1 int64 = 32767
)

// SubmitV1 is intentionally NOT a blockchain marketplace create-job request.
// Identity, payer wallet, reward, nonce, and priority must be server-owned.
// The V1 workload has fixed dimensions (3 features, 3 classes).
type SubmitV1 struct {
	BatchSize uint64  `json:"batch_size"`
	Inputs    []int64 `json:"inputs"`
	Weights   []int64 `json:"weights"`
	Biases    []int64 `json:"biases"`
}

// ParseSubmitV1 validates untrusted JSON and derives the actual Prism task.
// Only call with bytes from a separately size-limited HTTP request body.
func ParseSubmitV1(data []byte) (usefulwork.Task, error) {
	if len(data) == 0 || len(data) > MaxSubmitBodyBytesV1 {
		return usefulwork.Task{}, errors.New("invalid SaaS request body size")
	}

	// json.Decoder normally accepts duplicate keys; reject ambiguous input.
	if err := rejectDuplicateTopLevelKeys(data); err != nil {
		return usefulwork.Task{}, err
	}

	var req SubmitV1
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return usefulwork.Task{}, fmt.Errorf("invalid SaaS request JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return usefulwork.Task{}, errors.New("SaaS request must contain exactly one JSON object")
	}

	if req.BatchSize == 0 || req.BatchSize > MaxBatchSizeV1 {
		return usefulwork.Task{}, errors.New("unsupported SaaS batch size")
	}
	if uint64(len(req.Inputs)) != req.BatchSize*3 ||
		len(req.Weights) != 9 || len(req.Biases) != 3 {
		return usefulwork.Task{}, errors.New("invalid SaaS quantized 3x3 tensor shapes")
	}
	for _, v := range req.Inputs {
		if !boundedQuantizedV1(v) {
			return usefulwork.Task{}, errors.New("SaaS inputs exceed numeric limits")
		}
	}
	for _, v := range req.Weights {
		if !boundedQuantizedV1(v) {
			return usefulwork.Task{}, errors.New("SaaS weights exceed numeric limits")
		}
	}
	for _, v := range req.Biases {
		if !boundedQuantizedV1(v) {
			return usefulwork.Task{}, errors.New("SaaS biases exceed numeric limits")
		}
	}

	// Never trust a client-supplied ID or input_hash: compute them server-side.
	task, err := usefulwork.NewMLInferenceQuantizedTask(
		req.BatchSize, 3, 3, req.Inputs, req.Weights, req.Biases,
	)
	if err != nil {
		return usefulwork.Task{}, fmt.Errorf("invalid Prism task: %w", err)
	}
	if err := ValidateTaskV1(task); err != nil {
		return usefulwork.Task{}, err
	}
	return task, nil
}

// ValidateTaskV1 is defense in depth, including Prism's canonical task check.
func ValidateTaskV1(task usefulwork.Task) error {
	if task.Type != usefulwork.TaskTypeMLInferenceQuantized ||
		task.RowsA == 0 || task.RowsA > MaxBatchSizeV1 ||
		task.ColsA != 3 || task.ColsB != 3 {
		return errors.New("unsupported Prism Cloud v1 workload")
	}
	if err := usefulwork.ValidateTask(task); err != nil {
		return fmt.Errorf("Prism task validation failed: %w", err)
	}
	return nil
}

func boundedQuantizedV1(v int64) bool {
	return v >= -MaxAbsValueV1 && v <= MaxAbsValueV1
}

func rejectDuplicateTopLevelKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("SaaS request must be a JSON object")
	}
	seen := make(map[string]struct{}, 4)
	allowed := map[string]struct{}{
		"batch_size": {}, "inputs": {}, "weights": {}, "biases": {},
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid SaaS JSON property: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("invalid SaaS JSON object key")
		}
		if _, allowedKey := allowed[key]; !allowedKey {
			return errors.New("unsupported SaaS JSON object property")
		}
		if _, exists := seen[key]; exists {
			return errors.New("duplicate SaaS JSON object property")
		}
		seen[key] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("invalid SaaS JSON value: %w", err)
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("SaaS request has an invalid object terminator")
	}
	if len(seen) != len(allowed) {
		return errors.New("SaaS request missing required JSON properties")
	}
	return nil
}
