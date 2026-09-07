package blockchain

import (
	"testing"

	"prism/internal/consensus"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func makeReceivedBlockFixture(
	t *testing.T,
) (
	*Blockchain,
	Block,
	*consensus.ProofOfStake,
) {
	t.Helper()

	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	source, err := NewBlockchain(
		map[string]uint64{
			alice.Address: 100,
			bob.Address:   10,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := source.LockStake(
		alice.Address,
		10,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		alice.Address,
		10,
	); err != nil {
		t.Fatal(err)
	}

	receiverBlocks := append(
		[]Block(nil),
		source.Blocks...,
	)

	receiver := &Blockchain{
		Blocks: receiverBlocks,
		LockedStakes: map[string]uint64{
			alice.Address: 10,
		},
	}

	tx := transaction.New(
		alice.Address,
		bob.Address,
		1,
		0,
		alice.PublicKeyHex(),
	)

	if err := tx.Sign(
		alice.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	block, err := source.AddBlock(
		[]transaction.Transaction{tx},
		nil,
		alice.Address,
		pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	return receiver, block, pos
}

func TestAppendValidatedBlockPreservesRemoteBlock(
	t *testing.T,
) {
	receiver, block, pos :=
		makeReceivedBlockFixture(t)

	if err := receiver.AppendValidatedBlock(
		block,
		pos,
	); err != nil {
		t.Fatal(err)
	}

	if len(receiver.Blocks) != 2 {
		t.Fatalf(
			"expected 2 blocks, got %d",
			len(receiver.Blocks),
		)
	}

	got := receiver.Blocks[1]

	if got.Hash != block.Hash {
		t.Fatalf(
			"expected exact remote hash %s, got %s",
			block.Hash,
			got.Hash,
		)
	}

	if !receiver.ValidateChain(pos) {
		t.Fatal(
			"expected receiver chain to remain valid",
		)
	}
}

func TestAppendValidatedBlockRejectsTampering(
	t *testing.T,
) {
	receiver, block, pos :=
		makeReceivedBlockFixture(t)

	block.Reward++

	if err := receiver.AppendValidatedBlock(
		block,
		pos,
	); err == nil {
		t.Fatal(
			"expected tampered block to be rejected",
		)
	}

	if len(receiver.Blocks) != 1 {
		t.Fatalf(
			"expected receiver to remain at genesis, got %d blocks",
			len(receiver.Blocks),
		)
	}
}
