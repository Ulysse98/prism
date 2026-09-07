package p2p

import (
	"sort"
	"sync"
	"time"
)

type Peer struct {
	NodeID   string
	Address  string
	ChainID  string
	Height   uint64
	LastHash string
	LastSeen time.Time
}

type PeerAdvertisement struct {
	NodeID   string `json:"node_id"`
	Address  string `json:"address"`
	Height   uint64 `json:"height"`
	LastHash string `json:"last_hash"`
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
	b.UpsertAt(hello, hello.ListenAddr)
}

func (b *PeerBook) UpsertAt(
	hello HelloMessage,
	address string,
) {
	if hello.NodeID == "" || address == "" {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.peers == nil {
		b.peers = make(map[string]Peer)
	}

	b.peers[hello.NodeID] = Peer{
		NodeID:   hello.NodeID,
		Address:  address,
		ChainID:  hello.ChainID,
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

func peerAdvertisements(
	peers []Peer,
	chainID string,
	selfNodeID string,
	requesterNodeID string,
) []PeerAdvertisement {
	result := make([]PeerAdvertisement, 0)

	for _, peer := range peers {
		if peer.NodeID == "" ||
			peer.Address == "" ||
			peer.ChainID != chainID ||
			peer.NodeID == selfNodeID ||
			peer.NodeID == requesterNodeID {

			continue
		}

		result = append(
			result,
			PeerAdvertisement{
				NodeID:   peer.NodeID,
				Address:  peer.Address,
				Height:   peer.Height,
				LastHash: peer.LastHash,
			},
		)
	}

	return result
}
