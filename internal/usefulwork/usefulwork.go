package usefulwork

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	"prism/internal/wallet"
)

const TaskTypeSumSquares = "sum_squares"

type Task struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Values    []uint64 `json:"values"`
	InputHash string   `json:"input_hash"`
}

type Proof struct {
	ID         string `json:"id"`
	Task       Task   `json:"task"`
	Worker     string `json:"worker"`
	PublicKey  string `json:"public_key"`
	Result     uint64 `json:"result"`
	OutputHash string `json:"output_hash"`
	Score      uint64 `json:"score"`
	Signature  string `json:"signature"`
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

	if task.Type != TaskTypeSumSquares {
		return fmt.Errorf(
			"unsupported useful work task type: %s",
			task.Type,
		)
	}

	if len(task.Values) == 0 {
		return fmt.Errorf(
			"useful work task cannot be empty",
		)
	}

	expectedInputHash, err := calculateInputHash(
		task.Values,
	)
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

	var result uint64

	for _, value := range task.Values {
		if value != 0 &&
			value > math.MaxUint64/value {

			return 0, fmt.Errorf(
				"useful work multiplication overflow",
			)
		}

		square := value * value

		if result > math.MaxUint64-square {
			return 0, fmt.Errorf(
				"useful work addition overflow",
			)
		}

		result += square
	}

	return result, nil
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

	return uint64(len(task.Values))
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

	result, err := Compute(task)
	if err != nil {
		return Proof{}, err
	}

	proof := Proof{
		Task:       task,
		Worker:     worker.Address,
		PublicKey:  worker.PublicKeyHex(),
		Result:     result,
		OutputHash: calculateOutputHash(result),
		Score:      scoreForTask(task),
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

	expectedResult, err := Compute(
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

	expectedOutputHash := calculateOutputHash(
		proof.Result,
	)

	if proof.OutputHash != expectedOutputHash {
		return fmt.Errorf(
			"invalid useful work output hash",
		)
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
