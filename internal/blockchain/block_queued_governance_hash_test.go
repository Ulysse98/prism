package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
)

func queuedGovernanceHashTestBlock() Block {
	return Block{
		Height: 42,
		Timestamp: time.Date(
			2026,
			time.September,
			11,
			12,
			0,
			0,
			0,
			time.UTC,
		),
		PreviousHash: "previous-hash",
		Proposer:     "validator-a",
		Reward:       3,
		Transactions: []transaction.Transaction{},
		UsefulWork:   []usefulwork.Proof{},
	}
}

func TestCalculateHashCommitsAuthorityProposal(
	t *testing.T,
) {
	first :=
		reserved.NewAuthorityProposal(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			"authority-a",
		)

	second :=
		reserved.NewAuthorityProposal(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			"authority-b",
		)

	blockA :=
		queuedGovernanceHashTestBlock()

	blockA.AuthorityProposals =
		[]reserved.AuthorityProposal{
			first,
		}

	blockB :=
		queuedGovernanceHashTestBlock()

	blockB.AuthorityProposals =
		[]reserved.AuthorityProposal{
			second,
		}

	if CalculateHash(blockA) ==
		CalculateHash(blockB) {

		t.Fatal(
			"authority proposal mutation did not change block hash",
		)
	}
}

func TestCalculateHashCommitsAuthorityExecution(
	t *testing.T,
) {
	blockA :=
		queuedGovernanceHashTestBlock()

	blockA.AuthorityExecutions =
		[]reserved.AuthorityExecution{
			reserved.NewAuthorityExecution(
				"proposal-a",
			),
		}

	blockB :=
		queuedGovernanceHashTestBlock()

	blockB.AuthorityExecutions =
		[]reserved.AuthorityExecution{
			reserved.NewAuthorityExecution(
				"proposal-b",
			),
		}

	if CalculateHash(blockA) ==
		CalculateHash(blockB) {

		t.Fatal(
			"authority execution mutation did not change block hash",
		)
	}
}

func TestQueuedGovernanceHashCommitsExistingAuthorityChanges(
	t *testing.T,
) {
	proposal :=
		reserved.NewAuthorityProposal(
			"prism-test-chain",
			2,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			"authority-c",
		)

	changeA :=
		reserved.NewAuthorityChange(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			"authority-a",
		)

	changeB :=
		reserved.NewAuthorityChange(
			"prism-test-chain",
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			"authority-b",
		)

	blockA :=
		queuedGovernanceHashTestBlock()

	blockA.AuthorityChanges =
		[]reserved.AuthorityChange{
			changeA,
		}

	blockA.AuthorityProposals =
		[]reserved.AuthorityProposal{
			proposal,
		}

	blockB := blockA

	blockB.AuthorityChanges =
		[]reserved.AuthorityChange{
			changeB,
		}

	if CalculateHash(blockA) ==
		CalculateHash(blockB) {

		t.Fatal(
			"queued governance hash did not commit authority changes",
		)
	}
}