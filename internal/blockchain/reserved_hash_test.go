package blockchain

import (
	"strings"
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
)

func reservedHashTestBlock() Block {
	return Block{
		Height:       1,
		Timestamp:    time.Unix(1, 0).UTC(),
		PreviousHash: "previous",
		Proposer:     "proposer",
		Reward:       1,
	}
}

func reservedHashTestAuthorization() reserved.Authorization {
	return reserved.Authorization{
		ID:         "authorization-id",
		ChainID:    "prism-test-chain",
		Nonce:      1,
		Pool:       consensus.ReservedPoolTreasury,
		Recipient:  "recipient",
		Amount:     100,
		Authorizer: "authorizer",
		PublicKey:  "public-key",
		Signature:  "signature",
	}
}

func TestEmptyReservedAuthorizationsPreserveHash(
	t *testing.T,
) {
	nilBlock :=
		reservedHashTestBlock()

	emptyBlock :=
		nilBlock

	emptyBlock.ReservedAuthorizations =
		[]reserved.Authorization{}

	nilHash :=
		CalculateHash(nilBlock)

	emptyHash :=
		CalculateHash(emptyBlock)

	if nilHash != emptyHash {
		t.Fatal(
			"empty reserved authorizations changed legacy block hash",
		)
	}
}

func TestReservedAuthorizationChangesBlockHash(
	t *testing.T,
) {
	block :=
		reservedHashTestBlock()

	legacyHash :=
		CalculateHash(block)

	block.ReservedAuthorizations =
		[]reserved.Authorization{
			reservedHashTestAuthorization(),
		}

	reservedHash :=
		CalculateHash(block)

	if legacyHash == reservedHash {
		t.Fatal(
			"reserved authorization must change block hash",
		)
	}
}

func TestReservedAuthorizationContentsAreHashed(
	t *testing.T,
) {
	block :=
		reservedHashTestBlock()

	block.ReservedAuthorizations =
		[]reserved.Authorization{
			reservedHashTestAuthorization(),
		}

	originalHash :=
		CalculateHash(block)

	block.ReservedAuthorizations[0].Amount++

	tamperedHash :=
		CalculateHash(block)

	if originalHash == tamperedHash {
		t.Fatal(
			"reserved authorization contents must be committed by block hash",
		)
	}
}

func TestGenesisRejectsReservedAuthorizations(
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

	bc.Blocks[0].ReservedAuthorizations =
		[]reserved.Authorization{
			reservedHashTestAuthorization(),
		}

	if _, err :=
		bc.GetState(); err == nil {

		t.Fatal(
			"expected genesis reserved authorization to fail",
		)
	}
}

func TestNormalBlockRejectsReservedAuthorizationsBeforeActivation(
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

	block := Block{
		Height:       1,
		Timestamp:    time.Unix(2, 0).UTC(),
		PreviousHash: bc.Blocks[0].Hash,
		Proposer:     "validator",
		ReservedAuthorizations: []reserved.Authorization{
			reservedHashTestAuthorization(),
		},
	}

	block.Hash =
		CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)

	_, err =
		bc.GetState()

	if err == nil {
		t.Fatal(
			"expected reserved authorizations to remain disabled",
		)
	}

	if !strings.Contains(
		err.Error(),
		"reserved authorizations are not activated in consensus",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
