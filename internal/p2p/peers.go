package p2p

import (
	"sort"
	"sync"
	"time"
)

type Peer struct {
	NodeID   string
	Address  string
	Height   uint64
	LastHash string
	LastSeen time.Time
}

type PeerBook struct {
	mu    sync.RWMutex
	peers map[string]Peer
}

func NewPeerBook() *PeerBook {
	return &PeerBook{
		peers: make(map[string]Peer),
	}
}

func (b *PeerBook) Upsert(hello HelloMessage) {
	if hello.NodeID == "" || hello.ListenAddr == "" {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.peers == nil {
		b.peers = make(map[string]Peer)
	}

	b.peers[hello.NodeID] = Peer{
		NodeID:   hello.NodeID,
		Address:  hello.ListenAddr,
		Height:   hello.Height,
		LastHash: hello.LastHash,
		LastSeen: time.Now().UTC(),
	}
}

func (b *PeerBook) Remove(nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.peers, nodeID)
}

func (b *PeerBook) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.peers)
}

func (b *PeerBook) List() []Peer {
	b.mu.RLock()
	defer b.mu.RUnlock()

	peers := make([]Peer, 0, len(b.peers))

	for _, peer := range b.peers {
		peers = append(peers, peer)
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].NodeID < peers[j].NodeID
	})

	return peers
}
