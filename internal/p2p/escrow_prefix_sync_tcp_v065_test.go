package p2p

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"prism/internal/escrow"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestEscrowV065P2PTCPSynchronizesValidExtension(t *testing.T) {
	base, pos, alice, bob, proof, jobID :=
		escrowCompetingFixtureV065(t, 10)

	branchA := cloneEscrowTipV065(base)

	release, err := branchA.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}
	if release.Height != 3 {
		t.Fatal("expected RELEASE at height 3")
	}

	// B begins with exactly the same history as A; finality is not implied.
	branchB := cloneEscrowTipV065(branchA)

	tx := transaction.New(
		alice.Address,
		bob.Address,
		1,
		0,
		alice.PublicKeyHex(),
	)
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}

	extra, err := branchB.AddBlock(
		[]transaction.Transaction{tx},
		nil,
		alice.Address,
		pos,
	)
	if err != nil {
		t.Fatal(err)
	}
	if extra.Height != 4 {
		t.Fatal("expected extension at height 4")
	}

	if !branchA.ValidateChain(pos) ||
		!branchB.ValidateChain(pos) {
		t.Fatal("invalid chain fixture")
	}

	if len(branchA.Blocks) != 4 ||
		len(branchB.Blocks) != 5 {
		t.Fatal("incorrect history lengths")
	}

	for i := range branchA.Blocks {
		if branchA.Blocks[i].Hash != branchB.Blocks[i].Hash {
			t.Fatal("B does not extend A's history")
		}
	}

	dirA := t.TempDir()
	dirB := t.TempDir()

	wallets := map[string]*wallet.Wallet{
		"Alice": alice,
		"Bob":   bob,
	}

	if err := storage.Save(dirA, branchA, pos, wallets); err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(dirB, branchB, pos, wallets); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	nodeA := NewServer(
		"prefix-sync-node-a",
		"127.0.0.1:0",
		dirA, branchA, pos, wallets,
	)
	nodeB := NewServer(
		"prefix-sync-node-b",
		listener.Addr().String(),
		dirB, branchB, pos, wallets,
	)

	type snapshot struct {
		Chain  []byte
		Disk   []byte
		Height uint64
		Hash   string
		Alice  uint64
		Bob    uint64
		Supply uint64
	}

	capture := func(node *Server, dir string) snapshot {
		t.Helper()

		encoded, err := json.Marshal(node.Chain)
		if err != nil {
			t.Fatal(err)
		}
		disk, err := os.ReadFile(filepath.Join(dir, "chain.json"))
		if err != nil {
			t.Fatal(err)
		}
		available, err := node.Chain.AvailableBalanceOf(alice.Address)
		if err != nil {
			t.Fatal(err)
		}
		balance, err := node.Chain.BalanceOf(bob.Address)
		if err != nil {
			t.Fatal(err)
		}
		supply, err := node.Chain.TotalSupply()
		if err != nil {
			t.Fatal(err)
		}
		record, err := node.Chain.GetEscrowRecordV065(jobID)
		if err != nil || record.Status != escrow.Released {
			t.Fatalf("incorrect escrow: %+v %v", record, err)
		}

		tip := node.Chain.Blocks[len(node.Chain.Blocks)-1]
		return snapshot{
			Chain: encoded, Disk: disk,
			Height: tip.Height, Hash: tip.Hash,
			Alice: available, Bob: balance, Supply: supply,
		}
	}

	beforeA := capture(nodeA, dirA)
	beforeB := capture(nodeB, dirB)

	if beforeA.Height != 3 || beforeB.Height != 4 {
		t.Fatal("incorrect initial heights")
	}
	if beforeB.Bob != beforeA.Bob+1 {
		t.Fatal("expected additional transaction on B")
	}

	connectOnce := func() {
		t.Helper()

		done := make(chan error, 1)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			nodeB.handleIncoming(conn)
			done <- nil
		}()

		if err := nodeA.Connect(listener.Addr().String()); err != nil {
			t.Fatalf("valid synchronization rejected: %v", err)
		}

		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(12 * time.Second):
			t.Fatal("receiver did not finish")
		}
	}

	t.Log("A at height 3 requests B's valid height-4 extension")
	connectOnce()

	afterA := capture(nodeA, dirA)
	afterB := capture(nodeB, dirB)

	if !reflect.DeepEqual(afterA, beforeB) {
		t.Fatal("A did not persist exactly B's state")
	}
	if !reflect.DeepEqual(afterB, beforeB) {
		t.Fatal("B was unexpectedly modified")
	}

	loaded, loadedPoS, _, err := storage.Load(dirA)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ValidateChain(loadedPoS) {
		t.Fatal("restored synchronized chain invalid")
	}

	if len(loaded.Blocks) != 5 ||
		loaded.Blocks[4].Hash != extra.Hash {
		t.Fatal("persisted extension missing")
	}

	releases := 0
	for _, block := range loaded.Blocks {
		releases += len(block.EscrowReleases)
	}
	if releases != 1 {
		t.Fatal("escrow RELEASE duplicated during sync")
	}

	bobAfter, err := loaded.BalanceOf(bob.Address)
	if err != nil || bobAfter != beforeB.Bob {
		t.Fatal("worker balance changed during reload")
	}

	t.Logf(
		"valid extension adopted: height=%d hash=%s",
		afterA.Height, afterA.Hash,
	)

	t.Log("second synchronization must be idempotent")
	connectOnce()

	if !reflect.DeepEqual(afterA, capture(nodeA, dirA)) {
		t.Fatal("repeat synchronization modified A")
	}
	if !reflect.DeepEqual(beforeB, capture(nodeB, dirB)) {
		t.Fatal("repeat synchronization modified B")
	}

	t.Log("prefix synchronization, persistence, and retry verified")
}
