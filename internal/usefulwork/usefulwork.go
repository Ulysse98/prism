package usefulwork

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"prism/internal/wallet"
)

const (
	TaskTypeSumSquares       = "sum_squares"
	TaskTypeDotProduct       = "dot_product"
	TaskTypePrimeCount       = "prime_count"
	TaskTypeMatrixMultiply   = "matrix_multiply"
	TaskTypeImageConvolution = "image_convolution"
	TaskTypeMLInferenceBatch = "ml_inference_batch"
)

type Task struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Values    []uint64 `json:"values"`
	ValuesB   []uint64 `json:"values_b,omitempty"`
	RowsA     uint64   `json:"rows_a,omitempty"`
	ColsA     uint64   `json:"cols_a,omitempty"`
	ColsB     uint64   `json:"cols_b,omitempty"`
	InputHash string   `json:"input_hash"`
}

type Proof struct {
	ID           string   `json:"id"`
	Task         Task     `json:"task"`
	Worker       string   `json:"worker"`
	PublicKey    string   `json:"public_key"`
	Result       uint64   `json:"result"`
	ResultValues []uint64 `json:"result_values,omitempty"`
	OutputHash   string   `json:"output_hash"`
	Score        uint64   `json:"score"`
	Signature    string   `json:"signature"`
}

func NewSumSquaresTask(
	values []uint64,
) (Task, error) {

	if len(values) == 0 {
		return Task{}, fmt.Errorf(
			"useful work task cannot be empty",
		)
	}

	taskValues := make(
		[]uint64,
		len(values),
	)

	copy(taskValues, values)

	inputHash, err := calculateInputHash(
		taskValues,
	)
	if err != nil {
		return Task{}, err
	}

	task := Task{
		Type:      TaskTypeSumSquares,
		Values:    taskValues,
		InputHash: inputHash,
	}

	task.ID = CalculateTaskID(task)

	return task, nil
}

func calculateInputHash(
	values []uint64,
) (string, error) {

	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:]), nil
}

func CalculateTaskID(
	task Task,
) string {

	payload := fmt.Sprintf(
		"%s|%s",
		task.Type,
		task.InputHash,
	)

	hash := sha256.Sum256(
		[]byte(payload),
	)

	return hex.EncodeToString(hash[:])
}

func ValidateTask(
	task Task,
) error {

	var expectedInputHash string
	var err error

	switch task.Type {
	case TaskTypeSumSquares:
		if err = validateSingleVectorTask(task); err != nil {
			return err
		}

		expectedInputHash, err = calculateInputHash(
			task.Values,
		)

	case TaskTypeDotProduct:
		if err = validateDotProductTask(task); err != nil {
			return err
		}

		expectedInputHash, err =
			calculateDotProductInputHash(
				task.Values,
				task.ValuesB,
			)

	case TaskTypePrimeCount:
		if err = validateSingleVectorTask(task); err != nil {
			return err
		}

		expectedInputHash, err = calculateInputHash(
			task.Values,
		)

	case TaskTypeMatrixMultiply:
		if err = validateMatrixMultiplyTask(task); err != nil {
			return err
		}

		expectedInputHash, err =
			calculateMatrixMultiplyInputHash(
				task,
			)

	case TaskTypeImageConvolution:
		if err = validateImageConvolutionTask(task); err != nil {
			return err
		}

		expectedInputHash, err =
			calculateImageConvolutionInputHash(
				task,
			)

	case TaskTypeMLInferenceBatch:
		if err = validateMLInferenceBatchTask(task); err != nil {
			return err
		}

		expectedInputHash, err =
			calculateMLInferenceInputHash(
				task,
			)

	default:
		return fmt.Errorf(
			"unsupported useful work task type: %s",
			task.Type,
		)
	}

	if err != nil {
		return err
	}

	if task.InputHash != expectedInputHash {
		return fmt.Errorf(
			"invalid useful work input hash",
		)
	}

	if task.ID != CalculateTaskID(task) {
		return fmt.Errorf(
			"invalid useful work task ID",
		)
	}

	return nil
}

func Compute(
	task Task,
) (uint64, error) {

	if err := ValidateTask(task); err != nil {
		return 0, err
	}

	switch task.Type {
	case TaskTypeSumSquares:
		return computeSumSquares(
			task.Values,
		)

	case TaskTypeDotProduct:
		return computeDotProduct(
			task.Values,
			task.ValuesB,
		)

	case TaskTypePrimeCount:
		return computePrimeCount(
			task.Values,
		), nil

	case TaskTypeMatrixMultiply:
		return 0, fmt.Errorf(
			"matrix_multiply returns matrix output; use ComputeMatrix",
		)

	case TaskTypeImageConvolution:
		return 0, fmt.Errorf(
			"image_convolution returns vector output; use ComputeImageConvolution",
		)

	case TaskTypeMLInferenceBatch:
		return 0, fmt.Errorf(
			"ml_inference_batch returns vector output; use ComputeMLInferenceBatch",
		)

	default:
		return 0, fmt.Errorf(
			"unsupported useful work task type: %s",
			task.Type,
		)
	}
}

func calculateOutputHash(
	result uint64,
) string {

	payload := fmt.Sprintf(
		"%d",
		result,
	)

	hash := sha256.Sum256(
		[]byte(payload),
	)

	return hex.EncodeToString(hash[:])
}

func scoreForTask(
	task Task,
) uint64 {

	switch task.Type {
	case TaskTypeDotProduct:
		return uint64(len(task.Values)) +
			uint64(len(task.ValuesB))

	case TaskTypeMatrixMultiply:
		return matrixWorkUnits(task)

	case TaskTypeImageConvolution:
		return convolutionWorkUnits(task)

	case TaskTypeMLInferenceBatch:
		return mlInferenceWorkUnits(task)

	default:
		return uint64(len(task.Values))
	}
}

// WorkUnits returns the deterministic amount of useful work
// represented by a valid PoUW task.
func WorkUnits(
	task Task,
) (uint64, error) {
	if err := ValidateTask(task); err != nil {
		return 0, err
	}

	return scoreForTask(task), nil
}

func proofPayload(
	proof Proof,
) string {

	return fmt.Sprintf(
		"%s|%s|%s|%d|%s|%d",
		proof.Task.ID,
		proof.Worker,
		proof.PublicKey,
		proof.Result,
		proof.OutputHash,
		proof.Score,
	)
}

func CalculateProofID(
	proof Proof,
) string {

	hash := sha256.Sum256(
		[]byte(proofPayload(proof)),
	)

	return hex.EncodeToString(hash[:])
}

func Execute(
	task Task,
	worker *wallet.Wallet,
) (Proof, error) {

	if worker == nil {
		return Proof{}, fmt.Errorf(
			"worker wallet cannot be nil",
		)
	}

	if err := ValidateTask(task); err != nil {
		return Proof{}, err
	}

	proof := Proof{
		Task:      task,
		Worker:    worker.Address,
		PublicKey: worker.PublicKeyHex(),
		Score:     scoreForTask(task),
	}

	if task.Type == TaskTypeMatrixMultiply ||
		task.Type == TaskTypeImageConvolution ||
		task.Type == TaskTypeMLInferenceBatch {

		var resultValues []uint64
		var err error

		switch task.Type {
		case TaskTypeMatrixMultiply:
			resultValues, err =
				ComputeMatrix(task)

		case TaskTypeImageConvolution:
			resultValues, err =
				ComputeImageConvolution(task)

		case TaskTypeMLInferenceBatch:
			resultValues, err =
				ComputeMLInferenceBatch(task)
		}
		if err != nil {
			return Proof{}, err
		}

		outputHash, err :=
			calculateValuesOutputHash(
				resultValues,
			)
		if err != nil {
			return Proof{}, err
		}

		proof.ResultValues =
			resultValues

		proof.OutputHash =
			outputHash
	} else {
		result, err := Compute(task)
		if err != nil {
			return Proof{}, err
		}

		proof.Result = result

		proof.OutputHash =
			calculateOutputHash(
				result,
			)
	}

	proof.ID = CalculateProofID(proof)

	signature := ed25519.Sign(
		worker.PrivateKey,
		[]byte(proof.ID),
	)

	proof.Signature = hex.EncodeToString(
		signature,
	)

	return proof, nil
}

func VerifyProof(
	proof Proof,
) error {

	if err := ValidateTask(proof.Task); err != nil {
		return err
	}

	if proof.Worker == "" {
		return fmt.Errorf(
			"useful work worker cannot be empty",
		)
	}

	if proof.PublicKey == "" {
		return fmt.Errorf(
			"useful work public key cannot be empty",
		)
	}

	if proof.Task.Type == TaskTypeMatrixMultiply ||
		proof.Task.Type == TaskTypeImageConvolution ||
		proof.Task.Type == TaskTypeMLInferenceBatch {

		if proof.Result != 0 {
			return fmt.Errorf(
				"vector useful work must not contain scalar result",
			)
		}

		var expectedValues []uint64
		var err error

		switch proof.Task.Type {
		case TaskTypeMatrixMultiply:
			expectedValues, err =
				ComputeMatrix(
					proof.Task,
				)

		case TaskTypeImageConvolution:
			expectedValues, err =
				ComputeImageConvolution(
					proof.Task,
				)

		case TaskTypeMLInferenceBatch:
			expectedValues, err =
				ComputeMLInferenceBatch(
					proof.Task,
				)
		}
		if err != nil {
			return err
		}

		if !equalUint64Slices(
			proof.ResultValues,
			expectedValues,
		) {
			return fmt.Errorf(
				"invalid useful work vector result",
			)
		}

		expectedOutputHash, err :=
			calculateValuesOutputHash(
				expectedValues,
			)
		if err != nil {
			return err
		}

		if proof.OutputHash !=
			expectedOutputHash {

			return fmt.Errorf(
				"invalid useful work output hash",
			)
		}
	} else {
		if len(proof.ResultValues) != 0 {
			return fmt.Errorf(
				"scalar useful work must not contain vector result",
			)
		}

		expectedResult, err :=
			Compute(
				proof.Task,
			)
		if err != nil {
			return err
		}

		if proof.Result != expectedResult {
			return fmt.Errorf(
				"invalid useful work result: expected %d, got %d",
				expectedResult,
				proof.Result,
			)
		}

		expectedOutputHash :=
			calculateOutputHash(
				proof.Result,
			)

		if proof.OutputHash !=
			expectedOutputHash {

			return fmt.Errorf(
				"invalid useful work output hash",
			)
		}
	}

	expectedScore := scoreForTask(
		proof.Task,
	)

	if proof.Score != expectedScore {
		return fmt.Errorf(
			"invalid useful work score",
		)
	}

	publicKey, err := wallet.DecodePublicKey(
		proof.PublicKey,
	)
	if err != nil {
		return err
	}

	expectedAddress := wallet.AddressFromPublicKey(
		publicKey,
	)

	if expectedAddress != proof.Worker {
		return fmt.Errorf(
			"useful work public key does not own worker address",
		)
	}

	if proof.ID != CalculateProofID(proof) {
		return fmt.Errorf(
			"invalid useful work proof ID",
		)
	}

	signature, err := hex.DecodeString(
		proof.Signature,
	)
	if err != nil {
		return fmt.Errorf(
			"invalid useful work signature encoding: %w",
			err,
		)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf(
			"invalid useful work signature size",
		)
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(proof.ID),
		signature,
	) {
		return fmt.Errorf(
			"invalid useful work signature",
		)
	}

	return nil
}
