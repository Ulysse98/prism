package p2p

import "testing"

func TestPeerBookUpsertAndList(t *testing.T) {
	book := NewPeerBook()

	book.Upsert(HelloMessage{
		NodeID:     "node-b",
		ListenAddr: "127.0.0.1:7002",
		Height:     2,
		LastHash:   "hash-b",
	})

	book.Upsert(HelloMessage{
		NodeID:     "node-a",
		ListenAddr: "127.0.0.1:7001",
		Height:     3,
		LastHash:   "hash-a",
	})

	if book.Len() != 2 {
		t.Fatalf("expected 2 peers, got %d", book.Len())
	}

	peers := book.List()

	if peers[0].NodeID != "node-a" {
		t.Fatalf("expected node-a first, got %s", peers[0].NodeID)
	}

	if peers[1].NodeID != "node-b" {
		t.Fatalf("expected node-b second, got %s", peers[1].NodeID)
	}

	if peers[0].LastSeen.IsZero() {
		t.Fatal("expected LastSeen to be recorded")
	}

	book.Upsert(HelloMessage{
		NodeID:     "node-a",
		ListenAddr: "127.0.0.1:7011",
		Height:     4,
		LastHash:   "hash-a-new",
	})

	if book.Len() != 2 {
		t.Fatalf("upsert must not duplicate peers")
	}

	updated := book.List()[0]

	if updated.Address != "127.0.0.1:7011" {
		t.Fatalf("peer address was not updated")
	}

	if updated.Height != 4 {
		t.Fatalf("peer height was not updated")
	}
}

func TestPeerBookRemove(t *testing.T) {
	book := NewPeerBook()

	book.Upsert(HelloMessage{
		NodeID:     "node-a",
		ListenAddr: "127.0.0.1:7001",
	})

	book.Remove("node-a")

	if book.Len() != 0 {
		t.Fatalf("expected empty peer book")
	}
}
