package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestChainIDFromEmptyConfigCommitmentPreservesLegacy(
	t *testing.T,
) {
	genesisHash :=
		"genesis-hash"

	got, err :=
		MakeChainIDFromConfigCommitment(
			genesisHash,
			"",
		)

	if err != nil {
		t.Fatal(err)
	}

	expected :=
		MakeChainID(
			genesisHash,
		)

	if got != expected {
		t.Fatalf(
			"expected legacy chain ID %s, got %s",
			expected,
			got,
		)
	}
}

func TestChainIDFromCommitmentMatchesConfiguredChainID(
	t *testing.T,
) {
	config :=
		ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"prism_authority_a",
				},
			},
		}

	commitment, err :=
		config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	fromCommitment, err :=
		MakeChainIDFromConfigCommitment(
			"genesis-hash",
			commitment,
		)

	if err != nil {
		t.Fatal(err)
	}

	fromConfig, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			config,
		)

	if err != nil {
		t.Fatal(err)
	}

	if fromCommitment != fromConfig {
		t.Fatal(
			"config commitment chain ID does not match configured chain ID",
		)
	}
}

func TestChainIDChangesWithConfigCommitment(
	t *testing.T,
) {
	first, err :=
		MakeChainIDFromConfigCommitment(
			"genesis-hash",
			"commitment-a",
		)

	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		MakeChainIDFromConfigCommitment(
			"genesis-hash",
			"commitment-b",
		)

	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal(
			"different config commitments must produce different chain IDs",
		)
	}
}

func TestChainIDFromConfigCommitmentRejectsEmptyGenesis(
	t *testing.T,
) {
	if _, err :=
		MakeChainIDFromConfigCommitment(
			"",
			"commitment-a",
		); err == nil {

		t.Fatal(
			"expected empty genesis hash to fail",
		)
	}
}
