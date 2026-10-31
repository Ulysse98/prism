package p2p

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"prism/internal/escrow"
	"prism/internal/storage"
	"prism/internal/wallet"
)

func TestEscrowV065P2PTCPRejectsEqualHeightFork(t *testing.T) {
	base, pos, alice, bob, proof, jobID :=
		escrowCompetingFixtureV065(t, 10)

	branchA := cloneEscrowTipV065(base)
	branchB := cloneEscrowTipV065(base)

	blockA, err := branchA.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(3 * time.Millisecond)

	blockB, err := branchB.AddEscrowReleaseBlockV065(
		proof, alice.Address, pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	if blockA.Height != blockB.Height ||
		blockA.Hash == blockB.Hash {
		t.Fatal("expected equal-height competing forks")
	}

	if !branchA.ValidateChain(pos) || !branchB.ValidateChain(pos) {
		t.Fatal("invalid fork fixture")
	}

	wallets := map[string]*wallet.Wallet{
		"Alice": alice,
		"Bob":   bob,
	}

	dirA := t.TempDir()
	dirB := t.TempDir()

	if err := storage.Save(dirA, branchA, pos, wallets); err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(dirB, branchB, pos, wallets); err != nil {
		t.Fatal(err)
	}

	listenA, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listenA.Close()

	listenB, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listenB.Close()

	nodeA := NewServer(
		"escrow-tcp-fork-a",
		listenA.Addr().String(),
		dirA, branchA, pos, wallets,
	)
	nodeB := NewServer(
		"escrow-tcp-fork-b",
		listenB.Addr().String(),
		dirB, branchB, pos, wallets,
	)

	type snapshot struct {
		chainJSON   []byte
		diskJSON    []byte
		tipHash     string
		aliceFunds  uint64
		bobBalance  uint64
		totalSupply uint64
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
			t.Fatalf("incorrect escrow record: %+v, %v", record, err)
		}

		return snapshot{
			chainJSON:   encoded,
			diskJSON:    disk,
			tipHash:     node.Chain.Blocks[len(node.Chain.Blocks)-1].Hash,
			aliceFunds:  available,
			bobBalance:  balance,
			totalSupply: supply,
		}
	}

	beforeA := capture(nodeA, dirA)
	beforeB := capture(nodeB, dirB)

	if beforeA.tipHash == beforeB.tipHash {
		t.Fatal("forks are not divergent")
	}

	connectOnce := func(
		name string,
		initiator *Server,
		receiver *Server,
		listener net.Listener,
	) {
		t.Helper()

		done := make(chan error, 1)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			receiver.handleIncoming(conn)
			done <- nil
		}()

		err := initiator.Connect(listener.Addr().String())

		if err == nil ||
			!strings.Contains(err.Error(), "fork detected at height 3") {
			t.Fatalf(
				"%s: expected explicit fork rejection; got %v",
				name, err,
			)
		}

		if err := <-done; err != nil {
			t.Fatalf("%s: receiver failure: %v", name, err)
		}

		t.Logf("%s: %v", name, err)
	}

	t.Run("A_connects_to_B", func(t *testing.T) {
		connectOnce("A -> B", nodeA, nodeB, listenB)
	})
	t.Run("B_connects_to_A", func(t *testing.T) {
		connectOnce("B -> A", nodeB, nodeA, listenA)
	})

	verifyUnchanged := func(
		name string,
		node *Server,
		dir string,
		before snapshot,
	) {
		t.Helper()

		after := capture(node, dir)

		if !bytes.Equal(before.chainJSON, after.chainJSON) {
			t.Fatalf("%s: in-memory chain changed", name)
		}
		if !bytes.Equal(before.diskJSON, after.diskJSON) {
			t.Fatalf("%s: persisted chain changed", name)
		}
		if before.tipHash != after.tipHash ||
			before.aliceFunds != after.aliceFunds ||
			before.bobBalance != after.bobBalance ||
			before.totalSupply != after.totalSupply {
			t.Fatalf("%s: economic state changed", name)
		}

		loaded, loadedPoS, _, err := storage.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !loaded.ValidateChain(loadedPoS) {
			t.Fatalf("%s: persisted chain invalid", name)
		}

		last := loaded.Blocks[len(loaded.Blocks)-1]
		if last.Hash != before.tipHash {
			t.Fatalf("%s: persisted fork tip replaced", name)
		}

		t.Logf(
			"%s unchanged: height=%d hash=%s bob=%d",
			name, last.Height, last.Hash, after.bobBalance,
		)
	}

	verifyUnchanged("A", nodeA, dirA, beforeA)
	verifyUnchanged("B", nodeB, dirB, beforeB)

	t.Log("TCP fork detected in both directions; no chain or balance mutation")
}
