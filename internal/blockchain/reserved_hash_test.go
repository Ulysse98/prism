package blockchain

import (
	"testing"
	"time"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
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

func TestNormalBlockAcceptsReservedAuthorization(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	bc, err := NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := bc.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	bc.Config = ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authorizer.Address,
			},
		},
	}

	authorization :=
		signedReservedAuthorizationForBlockchain(
			t,
			bc,
			authorizer,
		)

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	block := Block{
		Height:       1,
		Timestamp:    time.Unix(2, 0).UTC(),
		PreviousHash: bc.Blocks[0].Hash,
		Proposer:     validator.Address,
		Reward:       rewardPolicy.ProposerReward,
		ReservedAuthorizations: []reserved.Authorization{
			authorization,
		},
	}

	block.Hash =
		CalculateHash(block)

	bc.Blocks = append(
		bc.Blocks,
		block,
	)

	state, err := bc.GetState()
	if err != nil {
		t.Fatal(err)
	}

	recipientBalance :=
		state.Balances[authorization.Recipient]

	if recipientBalance != authorization.Amount {
		t.Fatalf(
			"expected reserved recipient balance %d, got %d",
			authorization.Amount,
			recipientBalance,
		)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"expected chain with valid reserved authorization to validate",
		)
	}

	supply, err := bc.GetSupplyState()
	if err != nil {
		t.Fatal(err)
	}

	if supply.ReservedEmission != authorization.Amount {
		t.Fatalf(
			"expected reserved emission %d, got %d",
			authorization.Amount,
			supply.ReservedEmission,
		)
	}

	expectedLedger :=
		uint64(1000) +
			rewardPolicy.ProposerReward +
			authorization.Amount

	if supply.LedgerSupply != expectedLedger {
		t.Fatalf(
			"expected ledger supply %d, got %d",
			expectedLedger,
			supply.LedgerSupply,
		)
	}
}
