package usefulwork

import (
	"reflect"
	"testing"

	"prism/internal/wallet"
)

func TestExternalMLProofV2MatchesCPU(t *testing.T) {
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name                     string
		batch, features, classes uint64
		inputs, weights, biases  []int64
	}{
		{
			name:  "2x2",
			batch: 2, features: 2, classes: 2,
			inputs:  []int64{2, -1, -3, 4},
			weights: []int64{1, 0, 0, 1},
			biases:  []int64{0, 0},
		},
		{
			name:  "3x3",
			batch: 3, features: 3, classes: 3,
			inputs: []int64{
				2, -3, 1,
				-1, 4, 2,
				0, -2, 5,
			},
			weights: []int64{
				2, 1, -1,
				-1, -2, 2,
				1, 1, 1,
			},
			biases: []int64{1, -1, 0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task, err := NewMLInferenceQuantizedTask(
				tc.batch, tc.features, tc.classes,
				tc.inputs, tc.weights, tc.biases,
			)
			if err != nil {
				t.Fatal(err)
			}

			const jobID = "lab-job"
			const chainID = "lab-chain"
			const genesis = "lab-genesis"

			reference, err := ExecuteCompute(
				task, jobID, chainID, genesis, worker,
			)
			if err != nil {
				t.Fatal(err)
			}

			values := append(
				[]uint64(nil), reference.ResultValues...,
			)

			external, err := BuildComputeMLProofV2FromResult(
				task, jobID, chainID, genesis,
				worker, values,
			)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(reference, external) {
				t.Fatal("external and CPU proofs differ")
			}

			if err := VerifyProof(external); err != nil {
				t.Fatal(err)
			}

			if err := VerifyComputeProofContext(
				external, jobID, chainID, genesis,
			); err != nil {
				t.Fatal(err)
			}

			// Ensure the caller cannot mutate the signed
			// result through the original input slice.
			original := external.ResultValues[0]
			values[0] = (values[0] + 1) % tc.classes

			if external.ResultValues[0] != original {
				t.Fatal("result slice was not copied")
			}
		})
	}
}

func TestExternalMLProofV2RejectsBadInputs(t *testing.T) {
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	task, err := NewMLInferenceQuantizedTask(
		3, 3, 3,
		[]int64{2, -3, 1, -1, 4, 2, 0, -2, 5},
		[]int64{2, 1, -1, -1, -2, 2, 1, 1, 1},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}

	build := func(
		worker *wallet.Wallet,
		job, chain, genesis string,
		values []uint64,
	) (Proof, error) {
		return BuildComputeMLProofV2FromResult(
			task, job, chain, genesis,
			worker, values,
		)
	}

	if _, err := build(
		nil, "job", "chain", "genesis",
		[]uint64{1, 2, 1},
	); err == nil {
		t.Fatal("nil wallet accepted")
	}

	if _, err := build(
		worker, "", "chain", "genesis",
		[]uint64{1, 2, 1},
	); err == nil {
		t.Fatal("empty job ID accepted")
	}

	if _, err := build(
		worker, "job", "chain", "genesis",
		[]uint64{1},
	); err == nil {
		t.Fatal("invalid result length accepted")
	}

	if _, err := build(
		worker, "job", "chain", "genesis",
		[]uint64{1, 3, 1},
	); err == nil {
		t.Fatal("out-of-range class accepted")
	}

	// Deliberately wrong but structurally valid results
	// may be signed locally. Consensus MUST reject them.
	wrong, err := build(
		worker, "job", "chain", "genesis",
		[]uint64{0, 0, 0},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyProof(wrong); err == nil {
		t.Fatal("consensus accepted incorrect inference")
	}

	// The result hash is part of the signed commitment.
	correct, err := build(
		worker, "job", "chain", "genesis",
		[]uint64{1, 2, 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	correct.OutputHash = "tampered"

	if err := VerifyProof(correct); err == nil {
		t.Fatal("tampered output hash accepted")
	}
}
