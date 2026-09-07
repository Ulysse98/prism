package blockchain

import "testing"

func TestBlockchainChainIDUsesGenesisHash(
	t *testing.T,
) {
	bc := &Blockchain{
		Blocks: []Block{
			{
				Hash: "genesis-hash",
			},
		},
	}

	got, err :=
		bc.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	want :=
		MakeChainID(
			"genesis-hash",
		)

	if got != want {
		t.Fatalf(
			"expected chain ID %s, got %s",
			want,
			got,
		)
	}
}

func TestMakeChainIDIsDeterministic(
	t *testing.T,
) {
	first :=
		MakeChainID(
			"genesis-hash",
		)

	second :=
		MakeChainID(
			"genesis-hash",
		)

	if first != second {
		t.Fatal(
			"chain ID derivation must be deterministic",
		)
	}
}

func TestBlockchainChainIDRejectsMissingGenesis(
	t *testing.T,
) {
	bc :=
		&Blockchain{}

	if _, err :=
		bc.ChainID(); err == nil {

		t.Fatal(
			"expected missing genesis to fail",
		)
	}
}

func TestBlockchainChainIDRejectsEmptyGenesisHash(
	t *testing.T,
) {
	bc := &Blockchain{
		Blocks: []Block{
			{},
		},
	}

	if _, err :=
		bc.ChainID(); err == nil {

		t.Fatal(
			"expected empty genesis hash to fail",
		)
	}
}
