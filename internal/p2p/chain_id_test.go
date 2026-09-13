package p2p

import (
	"testing"

	"prism/internal/blockchain"
)

func TestMakeChainIDMatchesBlockchainIdentity(
	t *testing.T,
) {
	genesisHash :=
		"genesis-hash"

	got :=
		MakeChainID(
			genesisHash,
		)

	want :=
		blockchain.MakeChainID(
			genesisHash,
		)

	if got != want {
		t.Fatalf(
			"expected chain ID %s, got %s",
			want,
			got,
		)
	}
}
