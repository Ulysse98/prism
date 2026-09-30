package usefulwork

import (
	"encoding/json"
	"testing"

	"prism/internal/wallet"
)

func TestQuantumSimulationInputValid(t *testing.T) {
	raw := json.RawMessage(`{
"circuit": "bell",
"qubits": 2,
"shots": 4096
}`)

	if err := ValidateQuantumSimulationInput(raw); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestQuantumSimulationRejectsZeroShots(t *testing.T) {
	raw := json.RawMessage(`{
"circuit": "bell",
"qubits": 2,
"shots": 0
}`)

	if err := ValidateQuantumSimulationInput(raw); err == nil {
		t.Fatal("expected zero shots to be rejected")
	}
}

func TestQuantumSimulationRejectsMissingCircuit(t *testing.T) {
	raw := json.RawMessage(`{
"qubits": 2,
"shots": 4096
}`)

	if err := ValidateQuantumSimulationInput(raw); err == nil {
		t.Fatal("expected missing circuit to be rejected")
	}
}

func TestQuantumSimulationRejectsWrongBellQubitCount(t *testing.T) {
	raw := json.RawMessage(`{
"circuit": "bell",
"qubits": 3,
"shots": 4096
}`)

	if err := ValidateQuantumSimulationInput(raw); err == nil {
		t.Fatal("expected invalid Bell qubit count to be rejected")
	}
}

func TestVerifyBellResultAcceptsExpectedDistribution(t *testing.T) {
	result := QuantumSimulationResult{
		Backend:  "cuda-q",
		Workload: "bell",
		Shots:    4096,
		Counts: map[string]float64{
			"00": 0.501,
			"01": 0.000,
			"10": 0.000,
			"11": 0.499,
		},
	}

	if err := VerifyBellResult(result); err != nil {
		t.Fatalf("expected Bell result to be accepted, got %v", err)
	}
}

func TestVerifyBellResultRejectsInvalidDistribution(t *testing.T) {
	result := QuantumSimulationResult{
		Backend:  "cuda-q",
		Workload: "bell",
		Shots:    4096,
		Counts: map[string]float64{
			"00": 0.25,
			"01": 0.25,
			"10": 0.25,
			"11": 0.25,
		},
	}

	if err := VerifyBellResult(result); err == nil {
		t.Fatal("expected invalid Bell distribution to be rejected")
	}
}

func TestVerifyQuantumSimulationRejectsMissingBackend(t *testing.T) {
	result := QuantumSimulationResult{
		Workload: "bell",
		Shots:    4096,
		Counts: map[string]float64{
			"00": 0.5,
			"11": 0.5,
		},
	}

	if err := VerifyQuantumSimulationResult(result); err == nil {
		t.Fatal("expected missing backend to be rejected")
	}
}

func TestNewQuantumSimulationTask(t *testing.T) {
	task, err := NewQuantumSimulationTask(2, 4096)
	if err != nil {
		t.Fatal(err)
	}

	if task.Type != TaskTypeQuantumSimulation {
		t.Fatalf("unexpected task type: %s", task.Type)
	}

	if len(task.Values) != 2 {
		t.Fatalf("unexpected quantum values: %v", task.Values)
	}

	if task.Values[0] != 2 {
		t.Fatalf("unexpected qubit count: %d", task.Values[0])
	}

	if task.Values[1] != 4096 {
		t.Fatalf("unexpected shot count: %d", task.Values[1])
	}

	if task.InputHash == "" {
		t.Fatal("expected quantum input hash")
	}

	if task.ID == "" {
		t.Fatal("expected quantum task ID")
	}

	if err := ValidateTask(task); err != nil {
		t.Fatalf("expected quantum task to validate: %v", err)
	}
}

func TestQuantumSimulationTaskTamperRejected(t *testing.T) {
	task, err := NewQuantumSimulationTask(2, 4096)
	if err != nil {
		t.Fatal(err)
	}

	task.Values[1] = 8192

	if err := ValidateTask(task); err == nil {
		t.Fatal("expected tampered quantum task to be rejected")
	}
}

func TestQuantumSimulationRejectsExcessiveShots(t *testing.T) {
	_, err := NewQuantumSimulationTask(
		2,
		1_000_001,
	)

	if err == nil {
		t.Fatal("expected excessive quantum shots to be rejected")
	}
}

func TestQuantumSimulationWorkUnits(t *testing.T) {
	task, err := NewQuantumSimulationTask(2, 4096)
	if err != nil {
		t.Fatal(err)
	}

	workUnits, err := WorkUnits(task)
	if err != nil {
		t.Fatalf("WorkUnits failed: %v", err)
	}

	if workUnits == 0 {
		t.Fatal("quantum simulation work units must be greater than zero")
	}

	t.Logf("quantum simulation work units: %d", workUnits)
}

func TestVerifyBellCounts(t *testing.T) {
	counts := []uint64{
		2030,
		10,
		8,
		2048,
	}

	if err := VerifyBellCounts(
		counts,
		4096,
	); err != nil {
		t.Fatalf(
			"expected valid Bell counts: %v",
			err,
		)
	}
}

func TestVerifyBellCountsRejectsWrongTotal(t *testing.T) {
	counts := []uint64{
		2000,
		0,
		0,
		2000,
	}

	if err := VerifyBellCounts(
		counts,
		4096,
	); err == nil {
		t.Fatal(
			"expected Bell count total mismatch",
		)
	}
}

func TestVerifyBellCountsRejectsInvalidDistribution(t *testing.T) {
	counts := []uint64{
		1024,
		1024,
		1024,
		1024,
	}

	if err := VerifyBellCounts(
		counts,
		4096,
	); err == nil {
		t.Fatal(
			"expected invalid Bell distribution",
		)
	}
}

func TestComputeQuantumSimulation(t *testing.T) {
	task, err := NewQuantumSimulationTask(
		2,
		4096,
	)
	if err != nil {
		t.Fatal(err)
	}

	counts, err := ComputeQuantumSimulation(task)
	if err != nil {
		t.Fatal(err)
	}

	if len(counts) != 4 {
		t.Fatalf(
			"expected 4 Bell counts, got %d",
			len(counts),
		)
	}

	if counts[BellCount00]+
		counts[BellCount01]+
		counts[BellCount10]+
		counts[BellCount11] != 4096 {

		t.Fatal("Bell counts do not sum to shots")
	}

	if err := VerifyBellCounts(
		counts,
		4096,
	); err != nil {
		t.Fatalf(
			"generated Bell counts failed verification: %v",
			err,
		)
	}
}

func TestQuantumSimulationExecuteAndVerifyProof(t *testing.T) {
	task, err := NewQuantumSimulationTask(
		2,
		4096,
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := Execute(
		task,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if proof.Result != 0 {
		t.Fatalf(
			"expected quantum vector proof scalar result 0, got %d",
			proof.Result,
		)
	}

	if len(proof.ResultValues) != BellCountLength {
		t.Fatalf(
			"expected %d Bell counts, got %d",
			BellCountLength,
			len(proof.ResultValues),
		)
	}

	if err := VerifyBellCounts(
		proof.ResultValues,
		4096,
	); err != nil {
		t.Fatalf(
			"generated Bell counts invalid: %v",
			err,
		)
	}

	if proof.OutputHash == "" {
		t.Fatal(
			"expected quantum proof output hash",
		)
	}

	if proof.ID == "" {
		t.Fatal(
			"expected quantum proof ID",
		)
	}

	if proof.Signature == "" {
		t.Fatal(
			"expected quantum proof signature",
		)
	}

	if err := VerifyProof(proof); err != nil {
		t.Fatalf(
			"valid quantum proof rejected: %v",
			err,
		)
	}
}

func TestQuantumSimulationProofTamperRejected(t *testing.T) {
	task, err := NewQuantumSimulationTask(
		2,
		4096,
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := Execute(
		task,
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	proof.ResultValues[BellCount00] = 1024
	proof.ResultValues[BellCount01] = 1024
	proof.ResultValues[BellCount10] = 1024
	proof.ResultValues[BellCount11] = 1024

	if err := VerifyProof(proof); err == nil {
		t.Fatal(
			"expected tampered quantum proof to be rejected",
		)
	}
}

func TestQuantumSimulationExecuteComputeContextBound(t *testing.T) {
	task, err := NewQuantumSimulationTask(
		2,
		4096,
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := ExecuteCompute(
		task,
		"quantum-job-001",
		"prism-test-chain",
		"deadbeef-genesis",
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if proof.ProofVersion != ComputeProofVersion {
		t.Fatalf(
			"expected proof version %d, got %d",
			ComputeProofVersion,
			proof.ProofVersion,
		)
	}

	if proof.JobID != "quantum-job-001" {
		t.Fatalf(
			"unexpected job ID: %s",
			proof.JobID,
		)
	}

	if proof.ChainID != "prism-test-chain" {
		t.Fatalf(
			"unexpected chain ID: %s",
			proof.ChainID,
		)
	}

	if proof.GenesisHash != "deadbeef-genesis" {
		t.Fatalf(
			"unexpected genesis hash: %s",
			proof.GenesisHash,
		)
	}

	if err := VerifyProof(proof); err != nil {
		t.Fatalf(
			"valid context-bound quantum proof rejected: %v",
			err,
		)
	}

	if err := VerifyComputeProofContext(
		proof,
		"quantum-job-001",
		"prism-test-chain",
		"deadbeef-genesis",
	); err != nil {
		t.Fatalf(
			"valid quantum proof context rejected: %v",
			err,
		)
	}
}

func TestQuantumSimulationExecuteComputeWrongContextRejected(t *testing.T) {
	task, err := NewQuantumSimulationTask(
		2,
		4096,
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := ExecuteCompute(
		task,
		"quantum-job-001",
		"prism-test-chain",
		"deadbeef-genesis",
		worker,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyComputeProofContext(
		proof,
		"quantum-job-002",
		"prism-test-chain",
		"deadbeef-genesis",
	); err == nil {
		t.Fatal(
			"expected wrong quantum job context to be rejected",
		)
	}
}
