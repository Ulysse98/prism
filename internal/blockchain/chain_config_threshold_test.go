package blockchain

import (
	"testing"

	"prism/internal/reserved"
)

func TestChainConfigCommitmentIncludesReservedThreshold(
	t *testing.T,
) {
	base := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"authority-a",
				"authority-b",
				"authority-c",
			},
		},
	}

	threshold := base
	threshold.ReservedAuthorities.TreasuryThreshold = 2

	baseCommitment, err :=
		base.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	thresholdCommitment, err :=
		threshold.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if baseCommitment == thresholdCommitment {
		t.Fatal(
			"threshold must affect chain config commitment",
		)
	}
}

func TestChainConfigRejectsImpossibleReservedThreshold(
	t *testing.T,
) {
	config := ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				"authority-a",
			},
			TreasuryThreshold: 2,
		},
	}

	if _, err :=
		config.Commitment(); err == nil {

		t.Fatal(
			"expected impossible reserved threshold to fail",
		)
	}
}
