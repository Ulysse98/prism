package participation

import (
	"fmt"
	"math"
	"sort"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

const (
	ProposerPoints          uint64 = 10
	UsefulWorkPointMultiple uint64 = 2
)

// EligibilityChecker represents a source capable of determining
// whether a Prism address is eligible for Proof of Useful Participation.
type EligibilityChecker interface {
	IsVerified(address string) bool
}

type Score struct {
	Address            string `json:"address"`
	BlocksProposed     uint64 `json:"blocks_proposed"`
	UsefulWorkUnits    uint64 `json:"useful_work_units"`
	ProposerScore      uint64 `json:"proposer_score"`
	UsefulWorkScore    uint64 `json:"useful_work_score"`
	ParticipationScore uint64 `json:"participation_score"`
}

func Calculate(
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	eligibility EligibilityChecker,
) ([]Score, error) {

	if chain == nil {
		return nil, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if pos == nil {
		return nil, fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if eligibility == nil {
		return nil, fmt.Errorf(
			"participation eligibility checker cannot be nil",
		)
	}

	if !chain.ValidateChain(pos) {
		return nil, fmt.Errorf(
			"cannot calculate participation from invalid chain",
		)
	}

	scores := make(
		map[string]*Score,
	)

	for blockIndex, block := range chain.Blocks {
		if blockIndex == 0 {
			continue
		}

		// PoUP proposer points only count for humanity-verified addresses.
		if eligibility.IsVerified(block.Proposer) {
			proposerScore := getOrCreate(
				scores,
				block.Proposer,
			)

			if proposerScore.BlocksProposed == math.MaxUint64 {
				return nil, fmt.Errorf(
					"blocks proposed counter overflow",
				)
			}

			proposerScore.BlocksProposed++

			if proposerScore.ProposerScore >
				math.MaxUint64-ProposerPoints {

				return nil, fmt.Errorf(
					"proposer score overflow",
				)
			}

			proposerScore.ProposerScore += ProposerPoints
		}

		for _, proof := range block.UsefulWork {
			// Useful Work remains permissionless.
			// It contributes to PoUP only when the worker
			// has completed humanity verification.
			if !eligibility.IsVerified(proof.Worker) {
				continue
			}

			workerScore := getOrCreate(
				scores,
				proof.Worker,
			)

			if workerScore.UsefulWorkUnits >
				math.MaxUint64-proof.Score {

				return nil, fmt.Errorf(
					"useful work units overflow",
				)
			}

			workerScore.UsefulWorkUnits += proof.Score

			if proof.Score >
				math.MaxUint64/UsefulWorkPointMultiple {

				return nil, fmt.Errorf(
					"useful work participation score overflow",
				)
			}

			points := proof.Score *
				UsefulWorkPointMultiple

			if workerScore.UsefulWorkScore >
				math.MaxUint64-points {

				return nil, fmt.Errorf(
					"useful work score overflow",
				)
			}

			workerScore.UsefulWorkScore += points
		}
	}

	result := make(
		[]Score,
		0,
		len(scores),
	)

	for _, score := range scores {
		if score.ProposerScore >
			math.MaxUint64-score.UsefulWorkScore {

			return nil, fmt.Errorf(
				"participation score overflow",
			)
		}

		score.ParticipationScore =
			score.ProposerScore +
				score.UsefulWorkScore

		result = append(
			result,
			*score,
		)
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			if result[i].ParticipationScore ==
				result[j].ParticipationScore {

				return result[i].Address <
					result[j].Address
			}

			return result[i].ParticipationScore >
				result[j].ParticipationScore
		},
	)

	return result, nil
}

func ScoreOf(
	scores []Score,
	address string,
) uint64 {

	for _, score := range scores {
		if score.Address == address {
			return score.ParticipationScore
		}
	}

	return 0
}

func getOrCreate(
	scores map[string]*Score,
	address string,
) *Score {

	if existing, exists := scores[address]; exists {
		return existing
	}

	score := &Score{
		Address: address,
	}

	scores[address] = score

	return score
}
