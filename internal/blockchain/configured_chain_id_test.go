package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestConfiguredChainIDPreservesLegacyIdentity(
	t *testing.T,
) {
	genesisHash :=
		"genesis-hash"

	legacy :=
		MakeChainID(
			genesisHash,
		)

	configured, err :=
		MakeConfiguredChainID(
			genesisHash,
			DefaultChainConfig(),
		)

	if err != nil {
		t.Fatal(err)
	}

	if configured != legacy {
		t.Fatalf(
			"legacy chain ID changed: expected %s, got %s",
			legacy,
			configured,
		)
	}
}

func TestConfiguredChainIDChangesWithAuthorities(
	t *testing.T,
) {
	genesisHash :=
		"genesis-hash"

	legacy :=
		MakeChainID(
			genesisHash,
		)

	config := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_a",
			},
		},
	}

	configured, err :=
		MakeConfiguredChainID(
			genesisHash,
			config,
		)

	if err != nil {
		t.Fatal(err)
	}

	if configured == legacy {
		t.Fatal(
			"configured authorities must change chain identity",
		)
	}
}

func TestConfiguredChainIDIgnoresAuthorityOrder(
	t *testing.T,
) {
	firstConfig := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_b",
				"prism_authority_a",
			},
		},
	}

	secondConfig := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_a",
				"prism_authority_b",
			},
		},
	}

	first, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			firstConfig,
		)

	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			secondConfig,
		)

	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatal(
			"authority ordering changed configured chain ID",
		)
	}
}

func TestConfiguredChainIDChangesWithAuthorityPolicy(
	t *testing.T,
) {
	firstConfig := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_a",
			},
		},
	}

	secondConfig := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_b",
			},
		},
	}

	first, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			firstConfig,
		)

	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			secondConfig,
		)

	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal(
			"different authority policies must produce different chain IDs",
		)
	}
}

func TestConfiguredChainIDRejectsInvalidConfig(
	t *testing.T,
) {
	config := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Team: []string{
				"",
			},
		},
	}

	if _, err :=
		MakeConfiguredChainID(
			"genesis-hash",
			config,
		); err == nil {

		t.Fatal(
			"expected invalid chain config to fail",
		)
	}
}

func TestConfiguredChainIDRejectsEmptyGenesisHash(
	t *testing.T,
) {
	if _, err :=
		MakeConfiguredChainID(
			"",
			DefaultChainConfig(),
		); err == nil {

		t.Fatal(
			"expected empty genesis hash to fail",
		)
	}
}
