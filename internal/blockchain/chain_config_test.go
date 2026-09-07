package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestDefaultChainConfigIsLegacy(
	t *testing.T,
) {
	config :=
		DefaultChainConfig()

	if !config.IsLegacy() {
		t.Fatal(
			"default chain config must remain legacy compatible",
		)
	}
}

func TestChainConfigCommitmentIgnoresAuthorityOrder(
	t *testing.T,
) {
	first := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_b",
				"prism_authority_a",
			},
		},
	}

	second := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_a",
				"prism_authority_b",
			},
		},
	}

	firstCommitment, err :=
		first.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	secondCommitment, err :=
		second.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if firstCommitment !=
		secondCommitment {

		t.Fatal(
			"authority ordering must not change chain config commitment",
		)
	}
}

func TestChainConfigCommitmentChangesWithAuthority(
	t *testing.T,
) {
	first := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_a",
			},
		},
	}

	second := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_b",
			},
		},
	}

	firstCommitment, err :=
		first.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	secondCommitment, err :=
		second.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if firstCommitment ==
		secondCommitment {

		t.Fatal(
			"different authorities must change chain config commitment",
		)
	}
}

func TestChainConfigRejectsDuplicateAuthority(
	t *testing.T,
) {
	config := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_a",
				"prism_authority_a",
			},
		},
	}

	if _, err :=
		config.Commitment(); err == nil {

		t.Fatal(
			"expected duplicate authority to fail",
		)
	}
}

func TestChainConfigRejectsEmptyAuthority(
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
		config.Commitment(); err == nil {

		t.Fatal(
			"expected empty authority to fail",
		)
	}
}

func TestChainConfigRejectsGenesisAuthority(
	t *testing.T,
) {
	config := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Ecosystem: []string{
				"GENESIS",
			},
		},
	}

	if _, err :=
		config.Commitment(); err == nil {

		t.Fatal(
			"expected GENESIS authority to fail",
		)
	}
}

func TestChainConfigCanonicalDoesNotMutateInput(
	t *testing.T,
) {
	config := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"prism_authority_b",
				"prism_authority_a",
			},
		},
	}

	canonical, err :=
		config.Canonical()

	if err != nil {
		t.Fatal(err)
	}

	if config.ReservedAuthorities.
		Treasury[0] !=
		"prism_authority_b" {

		t.Fatal(
			"canonicalization mutated input config",
		)
	}

	if canonical.ReservedAuthorities.
		Treasury[0] !=
		"prism_authority_a" {

		t.Fatal(
			"canonical authority order is incorrect",
		)
	}
}
