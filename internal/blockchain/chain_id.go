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

func MakeConfiguredChainID(
	genesisHash string,
	config ChainConfig,
) (
	string,
	error,
) {
	if genesisHash == "" {
		return "",
			fmt.Errorf(
				"genesis block hash cannot be empty",
			)
	}

	if config.IsLegacy() {
		return MakeChainID(
			genesisHash,
		), nil
	}

	commitment, err :=
		config.Commitment()

	if err != nil {
		return "",
			fmt.Errorf(
				"invalid chain config: %w",
				err,
			)
	}

	hash :=
		sha256.Sum256(
			[]byte(
				"prism-chain-v2|" +
					genesisHash +
					"|" +
					commitment,
			),
		)

	return "prism-" +
		hex.EncodeToString(
			hash[:8],
		), nil
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
