package p2p

import (
	"net"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestReachablePeerAddressReplacesWildcardHost(t *testing.T) {
	remote := &net.TCPAddr{
		IP:   net.ParseIP("172.20.0.3"),
		Port: 54321,
	}

	got := reachablePeerAddress(
		"0.0.0.0:7003",
		remote,
	)

	if got != "172.20.0.3:7003" {
		t.Fatalf(
			"expected 172.20.0.3:7003, got %s",
			got,
		)
	}
}

func TestReachablePeerAddressPreservesAdvertisedHost(t *testing.T) {
	remote := &net.TCPAddr{
		IP:   net.ParseIP("172.20.0.3"),
		Port: 54321,
	}

	got := reachablePeerAddress(
		"prism-node-3:7003",
		remote,
	)

	if got != "prism-node-3:7003" {
		t.Fatalf(
			"expected advertised address to survive, got %s",
			got,
		)
	}
}

func TestNewServerInitializesMempool(t *testing.T) {
	server := NewServer(
		"node-test",
		"127.0.0.1:7001",
		"data/test",
		nil,
		nil,
		nil,
	)

	if server.Pool == nil {
		t.Fatal("expected server mempool to be initialized")
	}

	if server.MempoolCount() != 0 {
		t.Fatal("expected empty server mempool")
	}
}

func makeBlockAcceptanceFixture(
	t *testing.T,
) (
	*Server,
	blockchain.Block,
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

	source, err := blockchain.NewBlockchain(
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

	receiver := &blockchain.Blockchain{
		Blocks: append(
			[]blockchain.Block(nil),
			source.Blocks...,
		),
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

	server := NewServer(
		"receiver-node",
		"127.0.0.1:7002",
		t.TempDir(),
		receiver,
		pos,
		map[string]*wallet.Wallet{
			"Alice": alice,
			"Bob":   bob,
		},
	)

	return server, block
}

func TestAcceptBlockTreatsExactDuplicateAsKnown(
	t *testing.T,
) {
	server, block :=
		makeBlockAcceptanceFixture(t)

	appended, err := server.acceptBlock(
		block,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !appended {
		t.Fatal(
			"expected first block submission to append",
		)
	}

	appended, err = server.acceptBlock(
		block,
	)
	if err != nil {
		t.Fatal(err)
	}

	if appended {
		t.Fatal(
			"expected duplicate block to be treated as already known",
		)
	}

	if len(server.Chain.Blocks) != 2 {
		t.Fatalf(
			"expected exactly 2 blocks, got %d",
			len(server.Chain.Blocks),
		)
	}
}

func TestAcceptBlockRejectsConflictingKnownHeight(
	t *testing.T,
) {
	server, block :=
		makeBlockAcceptanceFixture(t)

	appended, err := server.acceptBlock(
		block,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !appended {
		t.Fatal(
			"expected first block submission to append",
		)
	}

	conflict := block
	conflict.Hash =
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	appended, err = server.acceptBlock(
		conflict,
	)

	if err == nil {
		t.Fatal(
			"expected conflicting block to be rejected",
		)
	}

	if appended {
		t.Fatal(
			"conflicting block must not append",
		)
	}

	if len(server.Chain.Blocks) != 2 {
		t.Fatalf(
			"conflict mutated chain: got %d blocks",
			len(server.Chain.Blocks),
		)
	}
}
