package blockchain

import (
	"testing"

	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestReservedEmissionIsZeroAtGenesis(
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

	emission, err :=
		bc.ReservedEmission()

	if err != nil {
		t.Fatal(err)
	}

	if emission != 0 {
		t.Fatalf(
			"expected zero explicit reserved emission, got %d",
			emission,
		)
	}
}

func TestReservedEmissionTracksChainAuthorizations(
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

	authorizer, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	bc.Config =
		ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					authorizer.Address,
				},
			},
		}

	first :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			1,
			100,
		)

	second :=
		signedReservedReplayAuthorization(
			t,
			bc,
			authorizer,
			2,
			250,
		)

	bc.Blocks = append(
		bc.Blocks,
		Block{
			Height: 1,
			ReservedAuthorizations: []reserved.Authorization{
				first,
			},
		},
		Block{
			Height: 2,
			ReservedAuthorizations: []reserved.Authorization{
				second,
			},
		},
	)

	emission, err :=
		bc.ReservedEmission()

	if err != nil {
		t.Fatal(err)
	}

	if emission != 350 {
		t.Fatalf(
			"expected reserved emission 350, got %d",
			emission,
		)
	}
}
