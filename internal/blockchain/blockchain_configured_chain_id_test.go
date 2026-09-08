package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestBlockchainConfiguredChainIDUsesChainConfig(
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

	bc :=
		&Blockchain{
			Blocks: []Block{
				{
					Hash: "genesis-hash",
				},
			},
			Config: config,
		}

	got, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	expected, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			config,
		)

	if err != nil {
		t.Fatal(err)
	}

	if got != expected {
		t.Fatalf(
			"expected configured Chain ID %s, got %s",
			expected,
			got,
		)
	}

	legacy :=
		MakeChainID(
			"genesis-hash",
		)

	if got == legacy {
		t.Fatal(
			"configured blockchain must not use legacy Chain ID",
		)
	}
}

func TestBlockchainConfiguredChainIDRejectsInvalidConfig(
	t *testing.T,
) {
	bc :=
		&Blockchain{
			Blocks: []Block{
				{
					Hash: "genesis-hash",
				},
			},
			Config: ChainConfig{
				ReservedAuthorities: reserved.AuthorityPolicy{
					Treasury: []string{
						"",
					},
				},
			},
		}

	if _, err :=
		bc.ChainID(); err == nil {

		t.Fatal(
			"invalid chain config must prevent Chain ID derivation",
		)
	}
}

func TestBlockchainConfiguredChainIDRejectsThresholdWithoutAuthorities(
	t *testing.T,
) {
	bc := &Blockchain{
		Blocks: []Block{
			{
				Hash: "genesis-hash",
			},
		},
		Config: ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				TreasuryThreshold: 2,
			},
		},
	}

	if _, err :=
		bc.ChainID(); err == nil {

		t.Fatal(
			"threshold without authorities must prevent Chain ID derivation",
		)
	}
}
