package main

import (
	"fmt"

	"prism/internal/blockchain"
)

func usefulWorkRewardsByProof(
	chain *blockchain.Blockchain,
) (map[string]uint64, error) {
	if chain == nil {
		return nil, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	rewards := make(map[string]uint64)

	var usefulWorkEmission uint64

	for blockIndex, block := range chain.Blocks {
		if blockIndex == 0 {
			continue
		}

		for _, proof := range block.UsefulWork {
			reward, err := mineRewardForWork(
				block.Height,
				proof.Score,
				usefulWorkEmission,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot calculate useful work reward for block %d proof %s: %w",
					block.Height,
					proof.ID,
					err,
				)
			}

			rewards[proof.ID] = reward
			usefulWorkEmission += reward
		}
	}

	return rewards, nil
}
