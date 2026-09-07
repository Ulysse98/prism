package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func chainConfigValidationFixture(
	t *testing.T,
) (
	*Blockchain,
	*consensus.ProofOfStake,
	*wallet.Wallet,
	*wallet.Wallet,
) {
	t.Helper()

	alice, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	bob, err :=
		wallet.New()

	if err != nil {
		t.Fatal(err)
	}

	bc, err :=
		NewBlockchain(
			map[string]uint64{
				alice.Address: 1000,
				bob.Address:   10,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if err :=
		bc.LockStake(
			alice.Address,
			100,
		); err != nil {

		t.Fatal(err)
	}

	pos :=
		consensus.NewProofOfStake()

	if err :=
		pos.Register(
			alice.Address,
			100,
		); err != nil {

		t.Fatal(err)
	}

	if !bc.ValidateChain(pos) {
		t.Fatal(
			"fixture chain must start valid",
		)
	}

	return bc, pos, alice, bob
}

func TestValidateChainRejectsInvalidChainConfig(
	t *testing.T,
) {
	bc, pos, _, _ :=
		chainConfigValidationFixture(t)

	bc.Config =
		ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"",
				},
			},
		}

	if bc.ValidateChain(pos) {
		t.Fatal(
			"invalid chain config must invalidate chain",
		)
	}
}

func TestAppendValidatedBlockUsesCandidateChainConfig(
	t *testing.T,
) {
	source, pos, alice, bob :=
		chainConfigValidationFixture(t)

	receiver :=
		&Blockchain{
			Blocks: append(
				[]Block(nil),
				source.Blocks...,
			),
			LockedStakes: map[string]uint64{
				alice.Address: 100,
			},
			Config: ChainConfig{
				ReservedAuthorities: reserved.AuthorityPolicy{
					Treasury: []string{
						"",
					},
				},
			},
		}

	tx :=
		transaction.New(
			alice.Address,
			bob.Address,
			1,
			0,
			alice.PublicKeyHex(),
		)

	if err :=
		tx.Sign(
			alice.PrivateKey,
		); err != nil {

		t.Fatal(err)
	}

	block, err :=
		source.AddBlock(
			[]transaction.Transaction{
				tx,
			},
			nil,
			alice.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if err :=
		receiver.AppendValidatedBlock(
			block,
			pos,
		); err == nil {

		t.Fatal(
			"candidate with invalid chain config must be rejected",
		)
	}

	if len(receiver.Blocks) != 1 {
		t.Fatal(
			"failed candidate mutated receiver chain",
		)
	}
}
