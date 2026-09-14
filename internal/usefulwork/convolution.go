package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

func NewImageConvolutionTask(
	rows uint64,
	cols uint64,
	kernelSize uint64,
	image []uint64,
	kernel []uint64,
) (Task, error) {

	task := Task{
		Type:    TaskTypeImageConvolution,
		Values:  cloneValues(image),
		ValuesB: cloneValues(kernel),
		RowsA:   rows,
		ColsA:   cols,
		ColsB:   kernelSize,
	}

	if err := validateImageConvolutionTask(task); err != nil {
		return Task{}, err
	}

	inputHash, err :=
		calculateImageConvolutionInputHash(task)
	if err != nil {
		return Task{}, err
	}

	task.InputHash = inputHash
	task.ID = CalculateTaskID(task)

	return task, nil
}

func validateImageConvolutionTask(
	task Task,
) error {

	if task.RowsA == 0 ||
		task.ColsA == 0 ||
		task.ColsB == 0 {

		return fmt.Errorf(
			"image convolution dimensions must be non-zero",
		)
	}

	kernelSize := task.ColsB

	if kernelSize > task.RowsA ||
		kernelSize > task.ColsA {

		return fmt.Errorf(
			"convolution kernel exceeds image dimensions",
		)
	}

	imageElements, err :=
		checkedMulUint64(
			task.RowsA,
			task.ColsA,
		)
	if err != nil {
		return err
	}

	kernelElements, err :=
		checkedMulUint64(
			kernelSize,
			kernelSize,
		)
	if err != nil {
		return err
	}

	outputRows :=
		task.RowsA - kernelSize + 1
	outputCols :=
		task.ColsA - kernelSize + 1

	outputElements, err :=
		checkedMulUint64(
			outputRows,
			outputCols,
		)
	if err != nil {
		return err
	}

	if imageElements > maxMatrixElements ||
		kernelElements > maxMatrixElements ||
		outputElements > maxMatrixElements {

		return fmt.Errorf(
			"image convolution task exceeds maximum size",
		)
	}

	if uint64(len(task.Values)) !=
		imageElements {

		return fmt.Errorf(
			"image expects %d values, got %d",
			imageElements,
			len(task.Values),
		)
	}

	if uint64(len(task.ValuesB)) !=
		kernelElements {

		return fmt.Errorf(
			"kernel expects %d values, got %d",
			kernelElements,
			len(task.ValuesB),
		)
	}

	_, err = convolutionWorkUnitsChecked(task)

	return err
}

func calculateImageConvolutionInputHash(
	task Task,
) (string, error) {

	payload := struct {
		Rows       uint64   `json:"rows"`
		Cols       uint64   `json:"cols"`
		KernelSize uint64   `json:"kernel_size"`
		Image      []uint64 `json:"image"`
		Kernel     []uint64 `json:"kernel"`
	}{
		Rows:       task.RowsA,
		Cols:       task.ColsA,
		KernelSize: task.ColsB,
		Image:      task.Values,
		Kernel:     task.ValuesB,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:]), nil
}

func ComputeImageConvolution(
	task Task,
) ([]uint64, error) {

	if err := ValidateTask(task); err != nil {
		return nil, err
	}

	if task.Type != TaskTypeImageConvolution {
		return nil, fmt.Errorf(
			"task type %s is not an image convolution",
			task.Type,
		)
	}

	return computeImageConvolution(task)
}

func computeImageConvolution(
	task Task,
) ([]uint64, error) {

	kernelSize := task.ColsB

	outputRows :=
		task.RowsA - kernelSize + 1
	outputCols :=
		task.ColsA - kernelSize + 1

	outputCount, err :=
		checkedMulUint64(
			outputRows,
			outputCols,
		)
	if err != nil {
		return nil, err
	}

	result := make(
		[]uint64,
		int(outputCount),
	)

	for outRow := uint64(0); outRow < outputRows; outRow++ {

		for outCol := uint64(0); outCol < outputCols; outCol++ {

			var cell uint64

			for kernelRow := uint64(0); kernelRow < kernelSize; kernelRow++ {

				for kernelCol := uint64(0); kernelCol < kernelSize; kernelCol++ {

					imageIndex :=
						(outRow+kernelRow)*
							task.ColsA +
							(outCol + kernelCol)

					kernelIndex :=
						kernelRow*kernelSize +
							kernelCol

					left :=
						task.Values[imageIndex]

					right :=
						task.ValuesB[kernelIndex]

					if right != 0 &&
						left >
							math.MaxUint64/right {

						return nil, fmt.Errorf(
							"image convolution multiplication overflow",
						)
					}

					product :=
						left * right

					if cell >
						math.MaxUint64-product {

						return nil, fmt.Errorf(
							"image convolution addition overflow",
						)
					}

					cell += product
				}
			}

			resultIndex :=
				outRow*outputCols +
					outCol

			result[resultIndex] = cell
		}
	}

	return result, nil
}

func convolutionWorkUnitsChecked(
	task Task,
) (uint64, error) {

	kernelSize := task.ColsB

	outputRows :=
		task.RowsA - kernelSize + 1
	outputCols :=
		task.ColsA - kernelSize + 1

	outputCells, err :=
		checkedMulUint64(
			outputRows,
			outputCols,
		)
	if err != nil {
		return 0, err
	}

	kernelCells, err :=
		checkedMulUint64(
			kernelSize,
			kernelSize,
		)
	if err != nil {
		return 0, err
	}

	return checkedMulUint64(
		outputCells,
		kernelCells,
	)
}

func convolutionWorkUnits(
	task Task,
) uint64 {

	score, err :=
		convolutionWorkUnitsChecked(task)

	if err != nil {
		return 0
	}

	return score
}
