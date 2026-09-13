package participation

import (
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

type alwaysEligible struct{}

func (alwaysEligible) IsVerifiedAtHeight(
	address string,
	height uint64,
) bool {
	return true
}

func buildPeriodTestChain(
	t *testing.T,
	blockCount int,
) (
	*blockchain.Blockchain,
	*consensus.ProofOfStake,
	*wallet.Wallet,
) {
	t.Helper()

	actor, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := blockchain.NewBlockchain(
		map[string]uint64{
			actor.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 100

	if err := chain.LockStake(
		actor.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		actor.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	for index := 1; index <= blockCount; index++ {
		task, err :=
			usefulwork.NewSumSquaresTask(
				[]uint64{
					uint64(index),
				},
			)

		if err != nil {
			t.Fatal(err)
		}

		proof, err :=
			usefulwork.Execute(
				task,
				actor,
			)

		if err != nil {
			t.Fatal(err)
		}

		if _, err := chain.AddBlock(
			nil,
			[]usefulwork.Proof{
				proof,
			},
			actor.Address,
			pos,
		); err != nil {
			t.Fatalf(
				"failed to add block %d: %v",
				index,
				err,
			)
		}
	}

	return chain, pos, actor
}

func TestCalculatePeriodSeparatesRewardPeriods(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPeriodTestChain(
			t,
			101,
		)

	periodZero, err := CalculatePeriod(
		chain,
		pos,
		alwaysEligible{},
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	periodOne, err := CalculatePeriod(
		chain,
		pos,
		alwaysEligible{},
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	scoreZero := ScoreOf(
		periodZero,
		actor.Address,
	)

	scoreOne := ScoreOf(
		periodOne,
		actor.Address,
	)

	// Each block contributes:
	// 10 proposer points + 2 useful-work points.
	if scoreZero != 1200 {
		t.Fatalf(
			"expected period 0 score 1200, got %d",
			scoreZero,
		)
	}

	if scoreOne != 12 {
		t.Fatalf(
			"expected period 1 score 12, got %d",
			scoreOne,
		)
	}
}

func TestCalculateStillUsesWholeChain(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPeriodTestChain(
			t,
			101,
		)

	scores, err := Calculate(
		chain,
		pos,
		alwaysEligible{},
	)
	if err != nil {
		t.Fatal(err)
	}

	score := ScoreOf(
		scores,
		actor.Address,
	)

	if score != 1212 {
		t.Fatalf(
			"expected lifetime score 1212, got %d",
			score,
		)
	}
}

func TestRewardUnitsForPeriodAddress(
	t *testing.T,
) {
	chain, pos, actor :=
		buildPeriodTestChain(
			t,
			101,
		)

	periodZero, err := CalculatePeriod(
		chain,
		pos,
		alwaysEligible{},
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	periodOne, err := CalculatePeriod(
		chain,
		pos,
		alwaysEligible{},
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if units :=
		RewardUnitsForAddress(
			periodZero,
			actor.Address,
		); units != 10 {

		t.Fatalf(
			"expected capped period 0 reward units 10, got %d",
			units,
		)
	}

	if units :=
		RewardUnitsForAddress(
			periodOne,
			actor.Address,
		); units != 1 {

		t.Fatalf(
			"expected period 1 reward units 1, got %d",
			units,
		)
	}
}
