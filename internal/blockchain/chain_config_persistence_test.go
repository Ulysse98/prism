package blockchain

import (
	"encoding/json"
	"testing"

	"prism/internal/reserved"
)

func TestNewBlockchainUsesDefaultChainConfig(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if !bc.Config.IsLegacy() {
		t.Fatal(
			"new blockchain must use legacy-compatible default config",
		)
	}
}

func TestLegacyBlockchainJSONWithoutConfigLoadsAsLegacy(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	legacy := struct {
		Blocks       []Block
		LockedStakes map[string]uint64
	}{
		Blocks:       bc.Blocks,
		LockedStakes: bc.LockedStakes,
	}

	data, err :=
		json.Marshal(
			legacy,
		)

	if err != nil {
		t.Fatal(err)
	}

	var restored Blockchain

	if err :=
		json.Unmarshal(
			data,
			&restored,
		); err != nil {

		t.Fatal(err)
	}

	if !restored.Config.IsLegacy() {
		t.Fatal(
			"missing config must decode as legacy config",
		)
	}

	originalChainID, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	restoredChainID, err :=
		restored.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	if restoredChainID !=
		originalChainID {

		t.Fatal(
			"legacy JSON changed chain identity",
		)
	}
}

func TestBlockchainJSONRoundTripPreservesChainConfig(
	t *testing.T,
) {
	bc, err :=
		NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	bc.Config =
		ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"prism_authority_b",
					"prism_authority_a",
				},
			},
		}

	before, err :=
		bc.Config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	data, err :=
		json.Marshal(
			bc,
		)

	if err != nil {
		t.Fatal(err)
	}

	var restored Blockchain

	if err :=
		json.Unmarshal(
			data,
			&restored,
		); err != nil {

		t.Fatal(err)
	}

	after, err :=
		restored.Config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if before != after {
		t.Fatal(
			"chain config commitment changed after JSON round trip",
		)
	}
}
