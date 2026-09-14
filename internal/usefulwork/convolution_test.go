package usefulwork

import (
	"testing"

	"prism/internal/wallet"
)

func TestImageConvolution(t *testing.T) {
	task, err := NewImageConvolutionTask(
		3,
		3,
		2,
		[]uint64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
		[]uint64{
			1, 0,
			0, 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ComputeImageConvolution(task)
	if err != nil {
		t.Fatal(err)
	}

	expected := []uint64{
		6, 8,
		12, 14,
	}

	if !equalUint64Slices(result, expected) {
		t.Fatalf(
			"expected %v, got %v",
			expected,
			result,
		)
	}

	units, err := WorkUnits(task)
	if err != nil {
		t.Fatal(err)
	}

	if units != 16 {
		t.Fatalf(
			"expected 16 work units, got %d",
			units,
		)
	}
}

func TestImageConvolutionProofRoundTrip(t *testing.T) {
	task, err := NewImageConvolutionTask(
		3,
		3,
		2,
		[]uint64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
		[]uint64{
			1, 0,
			0, 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := Execute(task, worker)
	if err != nil {
		t.Fatal(err)
	}

	if proof.Result != 0 {
		t.Fatalf(
			"convolution proof contains scalar result: %d",
			proof.Result,
		)
	}

	expected := []uint64{
		6, 8,
		12, 14,
	}

	if !equalUint64Slices(
		proof.ResultValues,
		expected,
	) {
		t.Fatalf(
			"unexpected convolution output: %v",
			proof.ResultValues,
		)
	}

	if proof.Score != 16 {
		t.Fatalf(
			"expected score 16, got %d",
			proof.Score,
		)
	}

	if err := VerifyProof(proof); err != nil {
		t.Fatalf(
			"valid convolution proof rejected: %v",
			err,
		)
	}
}

func TestImageConvolutionTamperRejected(t *testing.T) {
	task, err := NewImageConvolutionTask(
		2,
		2,
		2,
		[]uint64{
			1, 2,
			3, 4,
		},
		[]uint64{
			1, 1,
			1, 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	proof, err := Execute(task, worker)
	if err != nil {
		t.Fatal(err)
	}

	proof.ResultValues[0]++

	if err := VerifyProof(proof); err == nil {
		t.Fatal(
			"tampered convolution proof accepted",
		)
	}
}

func TestImageConvolutionHashCommitsKernel(t *testing.T) {
	task, err := NewImageConvolutionTask(
		2,
		2,
		2,
		[]uint64{
			1, 2,
			3, 4,
		},
		[]uint64{
			1, 0,
			0, 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	task.ValuesB[0]++

	if err := ValidateTask(task); err == nil {
		t.Fatal(
			"tampered convolution kernel accepted",
		)
	}
}

func TestImageConvolutionRejectsExcessiveWorkUnits(
	t *testing.T,
) {
	image := make(
		[]uint64,
		64*64,
	)

	kernel := make(
		[]uint64,
		32*32,
	)

	_, err :=
		NewImageConvolutionTask(
			64,
			64,
			32,
			image,
			kernel,
		)

	if err == nil {
		t.Fatal(
			"expected excessive convolution work units to be rejected",
		)
	}
}

func TestImageConvolutionWrongScoreRejected(
	t *testing.T,
) {
	task, err := NewImageConvolutionTask(
		3,
		3,
		2,
		[]uint64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
		[]uint64{
			1, 0,
			0, 1,
		},
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

	proof.Score++

	if err := VerifyProof(proof); err == nil {
		t.Fatal(
			"proof with tampered score accepted",
		)
	}
}
