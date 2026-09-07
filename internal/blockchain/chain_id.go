package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func MakeChainID(
	genesisHash string,
) string {
	hash := sha256.Sum256(
		[]byte(
			"prism-chain|" +
				genesisHash,
		),
	)

	return "prism-" +
		hex.EncodeToString(
			hash[:8],
		)
}

func (bc *Blockchain) ChainID() (
	string,
	error,
) {
	if bc == nil {
		return "",
			fmt.Errorf(
				"blockchain cannot be nil",
			)
	}

	if len(bc.Blocks) == 0 {
		return "",
			fmt.Errorf(
				"blockchain has no genesis block",
			)
	}

	genesisHash :=
		bc.Blocks[0].Hash

	if genesisHash == "" {
		return "",
			fmt.Errorf(
				"genesis block hash cannot be empty",
			)
	}

	return MakeChainID(
		genesisHash,
	), nil
}
