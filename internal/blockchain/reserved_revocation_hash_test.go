package blockchain

import (
	"testing"
	"time"

	"prism/internal/reserved"
)

func TestReservedRevocationParticipatesInBlockHash(
	t *testing.T,
) {
	block := Block{
		Height:       1,
		Timestamp:    time.Unix(100, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "validator",
		ReservedRevocations: []reserved.Revocation{
			{
				ID:      "revocation-id",
				ChainID: "chain-id",
				Pool:    "treasury",
				GrantID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	}

	first :=
		CalculateHash(block)

	block.ReservedRevocations[0].GrantID =
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	second :=
		CalculateHash(block)

	if first == second {
		t.Fatal(
			"reserved revocation mutation did not change block hash",
		)
	}
}

func TestLegacyBlockHashUnaffectedByEmptyRevocations(
	t *testing.T,
) {
	block := Block{
		Height:       1,
		Timestamp:    time.Unix(100, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "validator",
	}

	before :=
		CalculateHash(block)

	block.ReservedRevocations =
		[]reserved.Revocation{}

	after :=
		CalculateHash(block)

	if before != after {
		t.Fatal(
			"empty revocation slice changed legacy block hash",
		)
	}
}
