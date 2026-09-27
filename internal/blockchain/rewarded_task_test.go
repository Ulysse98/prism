package blockchain

import (
	"testing"

	"prism/internal/usefulwork"
)

func TestHasRewardedUsefulWorkTask(
	t *testing.T,
) {
	task, err := usefulwork.NewSumSquaresTask(
		[]uint64{7, 11, 13},
	)
	if err != nil {
		t.Fatal(err)
	}

	chain := &Blockchain{
		Blocks: []Block{
			{
				UsefulWork: []usefulwork.Proof{},
			},
		},
	}

	if chain.HasRewardedUsefulWorkTask(
		task.ID,
	) {
		t.Fatal(
			"unexpected rewarded task before proof exists",
		)
	}

	chain.Blocks = append(
		chain.Blocks,
		Block{
			UsefulWork: []usefulwork.Proof{
				{
					Task: task,
				},
			},
		},
	)

	if !chain.HasRewardedUsefulWorkTask(
		task.ID,
	) {
		t.Fatal(
			"expected rewarded task to be detected",
		)
	}

	if chain.HasRewardedUsefulWorkTask("") {
		t.Fatal(
			"empty task ID must not be considered rewarded",
		)
	}
}
