package usefulwork

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
)

const QuantumSimulationTaskType = "quantum_simulation"

const QuantumCircuitBell = "bell"

type QuantumSimulationInput struct {
	Circuit string `json:"circuit"`
	Qubits  int    `json:"qubits"`
	Shots   int    `json:"shots"`
}

type QuantumSimulationResult struct {
	Backend  string             `json:"backend"`
	Workload string             `json:"workload"`
	Shots    int                `json:"shots"`
	Counts   map[string]float64 `json:"counts"`
}

func ValidateQuantumSimulationInput(raw json.RawMessage) error {
	var input QuantumSimulationInput

	if err := json.Unmarshal(raw, &input); err != nil {
		return fmt.Errorf("decode quantum simulation input: %w", err)
	}

	if input.Circuit == "" {
		return errors.New("circuit is required")
	}

	if input.Circuit != QuantumCircuitBell {
		return fmt.Errorf("unsupported quantum circuit: %s", input.Circuit)
	}

	if input.Qubits <= 0 {
		return errors.New("qubits must be positive")
	}

	if input.Qubits != 2 {
		return errors.New("bell circuit requires exactly 2 qubits")
	}

	if input.Shots <= 0 {
		return errors.New("shots must be positive")
	}

	if input.Shots > 1_000_000 {
		return errors.New("quantum simulation shots exceed maximum")
	}

	return nil
}

func VerifyQuantumSimulationResult(result QuantumSimulationResult) error {
	if result.Backend == "" {
		return errors.New("backend is required")
	}

	if result.Workload == "" {
		return errors.New("workload is required")
	}

	if result.Shots <= 0 {
		return errors.New("shots must be positive")
	}

	if result.Counts == nil {
		return errors.New("counts are required")
	}

	switch result.Workload {
	case QuantumCircuitBell:
		return VerifyBellResult(result)
	default:
		return fmt.Errorf("unsupported quantum workload: %s", result.Workload)
	}
}

func VerifyBellResult(result QuantumSimulationResult) error {
	for _, state := range []string{"00", "01", "10", "11"} {
		p, ok := result.Counts[state]
		if !ok || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return fmt.Errorf("invalid Bell probability for %s: %v", state, p)
		}
	}

	const tolerance = 0.08

	p00 := result.Counts["00"]
	p01 := result.Counts["01"]
	p10 := result.Counts["10"]
	p11 := result.Counts["11"]

	total := p00 + p01 + p10 + p11

	if math.Abs(total-1.0) > 0.02 {
		return fmt.Errorf("invalid probability total: %f", total)
	}

	if math.Abs(p00-0.5) > tolerance {
		return fmt.Errorf("unexpected P(00): %f", p00)
	}

	if math.Abs(p11-0.5) > tolerance {
		return fmt.Errorf("unexpected P(11): %f", p11)
	}

	if p01 > tolerance {
		return fmt.Errorf("unexpected P(01): %f", p01)
	}

	if p10 > tolerance {
		return fmt.Errorf("unexpected P(10): %f", p10)
	}

	return nil
}

func NewQuantumSimulationTask(
	qubits uint64,
	shots uint64,
) (Task, error) {

	task := Task{
		Type: TaskTypeQuantumSimulation,
		Values: []uint64{
			qubits,
			shots,
		},
	}

	if err := validateQuantumSimulationTask(task); err != nil {
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

func validateQuantumSimulationTask(
	task Task,
) error {

	if len(task.Values) != 2 {
		return fmt.Errorf(
			"quantum_simulation requires [qubits, shots]",
		)
	}

	qubits := task.Values[0]
	shots := task.Values[1]

	if qubits != 2 {
		return fmt.Errorf(
			"bell quantum simulation requires exactly 2 qubits",
		)
	}

	if shots == 0 {
		return fmt.Errorf(
			"quantum simulation shots must be greater than zero",
		)
	}

	// Keep the first MVP bounded.
	if shots > 1_000_000 {
		return fmt.Errorf(
			"quantum simulation shots exceed maximum",
		)
	}

	if len(task.ValuesB) != 0 ||
		len(task.SignedValues) != 0 ||
		len(task.SignedValuesB) != 0 ||
		len(task.Biases) != 0 ||
		task.RowsA != 0 ||
		task.ColsA != 0 ||
		task.ColsB != 0 {

		return fmt.Errorf(
			"quantum_simulation contains unsupported task metadata",
		)
	}

	return nil
}

func QuantumSimulationParameters(
	task Task,
) (uint64, uint64, error) {

	if task.Type != TaskTypeQuantumSimulation {
		return 0, 0, fmt.Errorf(
			"task is not quantum_simulation",
		)
	}

	if err := validateQuantumSimulationTask(task); err != nil {
		return 0, 0, err
	}

	return task.Values[0], task.Values[1], nil
}

func quantumSimulationWorkUnits(
	task Task,
) uint64 {
	if len(task.Values) != 2 {
		return 0
	}

	return task.Values[1]
}

const (
	BellCount00 = iota
	BellCount01
	BellCount10
	BellCount11
	BellCountLength
)

func VerifyBellCounts(
	counts []uint64,
	shots uint64,
) error {

	if len(counts) != BellCountLength {
		return fmt.Errorf(
			"bell result requires exactly 4 counts",
		)
	}

	if shots == 0 {
		return fmt.Errorf(
			"bell result shots must be greater than zero",
		)
	}

	var total uint64

	for _, count := range counts {
		if ^uint64(0)-total < count {
			return fmt.Errorf(
				"bell count total overflow",
			)
		}

		total += count
	}

	if total != shots {
		return fmt.Errorf(
			"bell counts total %d does not match shots %d",
			total,
			shots,
		)
	}

	tolerance := float64(shots) * 0.08

	delta00 := math.Abs(
		float64(counts[BellCount00]) -
			float64(shots)/2.0,
	)

	delta11 := math.Abs(
		float64(counts[BellCount11]) -
			float64(shots)/2.0,
	)

	if delta00 > tolerance {
		return fmt.Errorf(
			"unexpected Bell 00 count: %d",
			counts[BellCount00],
		)
	}

	if delta11 > tolerance {
		return fmt.Errorf(
			"unexpected Bell 11 count: %d",
			counts[BellCount11],
		)
	}

	if float64(counts[BellCount01]) > tolerance {
		return fmt.Errorf(
			"unexpected Bell 01 count: %d",
			counts[BellCount01],
		)
	}

	if float64(counts[BellCount10]) > tolerance {
		return fmt.Errorf(
			"unexpected Bell 10 count: %d",
			counts[BellCount10],
		)
	}

	return nil
}

func ComputeQuantumSimulation(
	task Task,
) ([]uint64, error) {

	if os.Getenv("PRISM_QUANTUM_BACKEND") == "external" {
		return executeExternalQuantumSimulation(task)
	}

	_, shots, err := QuantumSimulationParameters(task)
	if err != nil {
		return nil, err
	}

	count00 := shots / 2
	count11 := shots - count00

	counts := []uint64{
		count00,
		0,
		0,
		count11,
	}

	if err := VerifyBellCounts(
		counts,
		shots,
	); err != nil {
		return nil, err
	}

	return counts, nil
}
