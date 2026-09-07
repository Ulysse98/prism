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

func TestPeerAdvertisementsFiltersNetworkAndRequester(t *testing.T) {
	peers := []Peer{
		{
			NodeID:  "node-requester",
			Address: "10.0.0.2:7002",
			ChainID: "prism-test",
		},
		{
			NodeID:   "node-b",
			Address:  "10.0.0.3:7003",
			ChainID:  "prism-test",
			Height:   8,
			LastHash: "hash-b",
		},
		{
			NodeID:  "node-other-network",
			Address: "10.0.0.4:7004",
			ChainID: "prism-other",
		},
		{
			NodeID:  "node-self",
			Address: "10.0.0.1:7001",
			ChainID: "prism-test",
		},
	}

	got := peerAdvertisements(
		peers,
		"prism-test",
		"node-self",
		"node-requester",
	)

	if len(got) != 1 {
		t.Fatalf(
			"expected 1 advertised peer, got %d",
			len(got),
		)
	}

	if got[0].NodeID != "node-b" {
		t.Fatalf(
			"expected node-b, got %s",
			got[0].NodeID,
		)
	}

	if got[0].Address != "10.0.0.3:7003" {
		t.Fatalf(
			"unexpected address: %s",
			got[0].Address,
		)
	}
}

func TestPeerBookHas(t *testing.T) {
	book := NewPeerBook()

	book.Upsert(HelloMessage{
		NodeID:     "node-a",
		ListenAddr: "127.0.0.1:7001",
		ChainID:    "prism-test",
	})

	if !book.Has("node-a") {
		t.Fatal("expected node-a to exist")
	}

	if book.Has("node-b") {
		t.Fatal("did not expect node-b to exist")
	}
}
