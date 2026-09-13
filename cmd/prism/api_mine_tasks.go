package main

import (
	"fmt"
	"strings"

	"prism/internal/usefulwork"
)

type mineTaskOption struct {
	Task       usefulwork.Task
	Difficulty string
}

func mineTaskCatalogForHeight(
	height uint64,
) ([]mineTaskOption, error) {

	base := (height % 97) + 11

	sumSquares, err :=
		usefulwork.NewSumSquaresTask(
			[]uint64{
				base,
				base + 3,
				base + 7,
			},
		)
	if err != nil {
		return nil, err
	}

	dotProduct, err :=
		usefulwork.NewDotProductTask(
			[]uint64{
				base,
				base + 2,
				base + 5,
			},
			[]uint64{
				2,
				3,
				5,
			},
		)
	if err != nil {
		return nil, err
	}

	primeCount, err :=
		usefulwork.NewPrimeCountTask(
			[]uint64{
				base,
				base + 1,
				base + 2,
				base + 3,
				base + 4,
				base + 5,
				base + 6,
				base + 7,
			},
		)
	if err != nil {
		return nil, err
	}

	matrixMultiply, err :=
		usefulwork.NewMatrixMultiplyTask(
			2,
			3,
			2,
			[]uint64{
				base,
				base + 1,
				base + 2,
				base + 3,
				base + 4,
				base + 5,
			},
			[]uint64{
				2, 3,
				5, 7,
				11, 13,
			},
		)
	if err != nil {
		return nil, err
	}

	return []mineTaskOption{
		{
			Task:       sumSquares,
			Difficulty: "LOW",
		},
		{
			Task:       dotProduct,
			Difficulty: "MEDIUM",
		},
		{
			Task:       primeCount,
			Difficulty: "MEDIUM",
		},
		{
			Task:       matrixMultiply,
			Difficulty: "HIGH",
		},
	}, nil
}

func mineTaskForHeightAndType(
	height uint64,
	taskType string,
) (mineTaskOption, error) {

	taskType = strings.TrimSpace(
		taskType,
	)

	// Backwards compatibility:
	// old clients send no task selector.
	if taskType == "" {
		taskType =
			usefulwork.TaskTypeSumSquares
	}

	options, err :=
		mineTaskCatalogForHeight(height)
	if err != nil {
		return mineTaskOption{}, err
	}

	for _, option := range options {
		if option.Task.Type == taskType {
			return option, nil
		}
	}

	return mineTaskOption{}, fmt.Errorf(
		"unsupported PoUW task type: %s",
		taskType,
	)
}

func mineTaskForJobID(
	height uint64,
	jobID string,
) (mineTaskOption, error) {

	jobID = strings.TrimSpace(jobID)

	if jobID == "" {
		return mineTaskOption{},
			fmt.Errorf(
				"PoUW job ID is required",
			)
	}

	options, err :=
		mineTaskCatalogForHeight(height)
	if err != nil {
		return mineTaskOption{}, err
	}

	for _, option := range options {
		if option.Task.ID == jobID {
			return option, nil
		}
	}

	return mineTaskOption{},
		fmt.Errorf(
			"invalid PoUW job ID",
		)
}
