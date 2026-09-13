package blockchain

import (
	"encoding/json"
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func TestCalculateHashCommitsReservedTransferProposal(
	t *testing.T,
) {
	proposal :=
		reserved.NewReservedTransferProposal(
			"prism-devnet",
			1,
			consensus.ReservedPoolTreasury,
			"recipient",
			100,
		)

	block := Block{
		Height:       100,
		Timestamp:    time.Unix(1_700_000_000, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "validator",
		Reward:       4,
		ReservedTransferProposals: []reserved.ReservedTransferProposal{
			proposal,
		},
	}

	first := CalculateHash(block)

	block.ReservedTransferProposals[0].Amount =
		101

	second := CalculateHash(block)

	if first == second {
		t.Fatal(
			"reserved transfer proposal amount was not committed to block hash",
		)
	}
}

func TestCalculateHashCommitsReservedTransferExecution(
	t *testing.T,
) {
	block := Block{
		Height:       105,
		Timestamp:    time.Unix(1_700_000_000, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "validator",
		Reward:       4,
		ReservedTransferExecutions: []reserved.ReservedTransferExecution{
			reserved.NewReservedTransferExecution(
				"proposal-a",
			),
		},
	}

	first := CalculateHash(block)

	block.ReservedTransferExecutions[0].ProposalID =
		"proposal-b"

	second := CalculateHash(block)

	if first == second {
		t.Fatal(
			"reserved transfer execution was not committed to block hash",
		)
	}
}

func TestCalculateHashAuthorityOnlyRetainsQueuedGovernanceV1(
	t *testing.T,
) {
	block := Block{
		Height:       100,
		Timestamp:    time.Unix(1_700_000_000, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "validator",
		Reward:       4,
		AuthorityExecutions: []reserved.AuthorityExecution{
			reserved.NewAuthorityExecution(
				"authority-proposal",
			),
		},
	}

	transactionData, err :=
		json.Marshal(
			block.Transactions,
		)
	if err != nil {
		t.Fatal(err)
	}

	usefulWorkData, err :=
		json.Marshal(
			block.UsefulWork,
		)
	if err != nil {
		t.Fatal(err)
	}

	expected :=
		calculateQueuedGovernanceBlockHash(
			block,
			transactionData,
			usefulWorkData,
		)

	actual :=
		CalculateHash(block)

	if actual != expected {
		t.Fatalf(
			"authority-only queued governance hash changed: got=%s expected=%s",
			actual,
			expected,
		)
	}
}

func TestCalculateHashMixedGovernanceUsesV2(
	t *testing.T,
) {
	block := Block{
		Height:       105,
		Timestamp:    time.Unix(1_700_000_000, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "validator",
		Reward:       4,
		AuthorityExecutions: []reserved.AuthorityExecution{
			reserved.NewAuthorityExecution(
				"authority-proposal",
			),
		},
		ReservedTransferExecutions: []reserved.ReservedTransferExecution{
			reserved.NewReservedTransferExecution(
				"transfer-proposal",
			),
		},
	}

	transactionData, err :=
		json.Marshal(
			block.Transactions,
		)
	if err != nil {
		t.Fatal(err)
	}

	usefulWorkData, err :=
		json.Marshal(
			block.UsefulWork,
		)
	if err != nil {
		t.Fatal(err)
	}

	expected :=
		calculateQueuedGovernanceBlockHashV2(
			block,
			transactionData,
			usefulWorkData,
		)

	actual :=
		CalculateHash(block)

	if actual != expected {
		t.Fatal(
			"mixed governance block did not use v2 hash domain",
		)
	}
}
