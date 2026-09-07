package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/storage"
	"prism/internal/wallet"
)

const ProtocolVersion = "0.19"

const (
	MessageHello    = "hello"
	MessageGetState = "get_state"
	MessageState    = "state"
	MessageGetPeers = "get_peers"
	MessagePeers    = "peers"
)

type HelloMessage struct {
	Type        string `json:"type"`
	Version     string `json:"version"`
	NodeID      string `json:"node_id"`
	ListenAddr  string `json:"listen_addr"`
	ChainID     string `json:"chain_id"`
	GenesisHash string `json:"genesis_hash"`
	Height      uint64 `json:"height"`
	LastHash    string `json:"last_hash"`
}

type StateRequest struct {
	Type string `json:"type"`
}

type StateResponse struct {
	Type       string                 `json:"type"`
	ChainID    string                 `json:"chain_id"`
	Blockchain *blockchain.Blockchain `json:"blockchain"`
	Validators []consensus.Validator  `json:"validators"`
}

type PeersResponse struct {
	Type    string              `json:"type"`
	ChainID string              `json:"chain_id"`
	Peers   []PeerAdvertisement `json:"peers"`
}

type Server struct {
	NodeID     string
	ListenAddr string
	DataDir    string

	Chain   *blockchain.Blockchain
	PoS     *consensus.ProofOfStake
	Wallets map[string]*wallet.Wallet
	Peers   *PeerBook

	dialMu  sync.Mutex
	dialing map[string]struct{}

	mu sync.RWMutex
}

func NewServer(
	nodeID string,
	listenAddr string,
	dataDir string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) *Server {
	return &Server{
		NodeID:     nodeID,
		ListenAddr: listenAddr,
		DataDir:    dataDir,
		Chain:      chain,
		PoS:        pos,
		Wallets:    wallets,
		Peers:      NewPeerBook(),
		dialing:    make(map[string]struct{}),
	}
}

func MakeNodeID(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:8])
}

func MakeChainID(genesisHash string) string {
	hash := sha256.Sum256(
		[]byte("prism-chain|" + genesisHash),
	)

	return "prism-" + hex.EncodeToString(hash[:8])
}

func (s *Server) Run(peer string) error {
	if s.Chain == nil {
		return fmt.Errorf("blockchain cannot be nil")
	}

	if s.PoS == nil {
		return fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if len(s.Wallets) == 0 {
		return fmt.Errorf(
			"wallet set cannot be empty",
		)
	}

	if !s.Chain.ValidateChain(s.PoS) {
		return fmt.Errorf(
			"refusing to start with invalid blockchain",
		)
	}

	listener, err := net.Listen(
		"tcp",
		s.ListenAddr,
	)
	if err != nil {
		return err
	}

	defer listener.Close()

	status := s.hello()

	fmt.Println("=== P2P NODE ===")
	fmt.Println("Node ID:", s.NodeID)
	fmt.Println("Listening:", s.ListenAddr)
	fmt.Println("Protocol:", ProtocolVersion)
	fmt.Println("Chain ID:", status.ChainID)
	fmt.Println("Height:", status.Height)
	fmt.Println(
		"Genesis:",
		shortHash(status.GenesisHash),
	)
	fmt.Println(
		"Last hash:",
		shortHash(status.LastHash),
	)

	errCh := make(chan error, 1)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				errCh <- err
				return
			}

			go s.handleIncoming(conn)
		}
	}()

	if peer != "" {
		fmt.Println()
		fmt.Println(
			"Connecting to peer:",
			peer,
		)

		if err := s.Connect(peer); err != nil {
			return fmt.Errorf(
				"peer connection failed: %w",
				err,
			)
		}

		go s.syncPeerLoop(peer)
	}

	fmt.Println()
	fmt.Println(
		"P2P node running. Press Ctrl+C to stop.",
	)

	return <-errCh
}

func (s *Server) syncPeerLoop(
	peer string,
) {
	ticker := time.NewTicker(
		5 * time.Second,
	)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.Connect(peer); err != nil {
			fmt.Println()
			fmt.Println(
				"Background peer sync failed:",
				err,
			)
		}
	}
}

func (s *Server) Connect(
	address string,
) error {
	return s.connect(
		address,
		"",
	)
}

func (s *Server) connect(
	address string,
	expectedNodeID string,
) error {
	conn, err := net.DialTimeout(
		"tcp",
		address,
		5*time.Second,
	)
	if err != nil {
		return err
	}

	defer conn.Close()

	if err := conn.SetDeadline(
		time.Now().Add(10 * time.Second),
	); err != nil {
		return err
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	localBefore := s.hello()

	if err := encoder.Encode(localBefore); err != nil {
		return err
	}

	var remote HelloMessage

	if err := decoder.Decode(&remote); err != nil {
		return err
	}

	if err := validateHello(remote); err != nil {
		return err
	}

	if remote.NodeID == s.NodeID {
		return fmt.Errorf(
			"refusing self connection",
		)
	}

	if expectedNodeID != "" &&
		remote.NodeID != expectedNodeID {

		return fmt.Errorf(
			"discovered peer identity mismatch: expected=%s received=%s",
			expectedNodeID,
			remote.NodeID,
		)
	}

	if expectedNodeID != "" &&
		remote.ChainID != localBefore.ChainID {

		return fmt.Errorf(
			"discovered peer network mismatch: local=%s remote=%s",
			localBefore.ChainID,
			remote.ChainID,
		)
	}

	s.Peers.UpsertAt(
		remote,
		address,
	)

	fmt.Println("Handshake accepted.")
	s.printPeer(remote)

	if remote.ChainID != localBefore.ChainID {
		if localBefore.Height != 0 {
			return fmt.Errorf(
				"chain ID mismatch: local=%s remote=%s",
				localBefore.ChainID,
				remote.ChainID,
			)
		}

		fmt.Println()
		fmt.Println(
			"Different network genesis detected.",
		)
		fmt.Println(
			"Local node only has its bootstrap Genesis.",
		)
		fmt.Println(
			"Requesting authoritative peer state...",
		)

		if err := s.requestAndAdoptState(
			encoder,
			decoder,
			remote,
			true,
		); err != nil {
			return err
		}

	} else if remote.Height > localBefore.Height {
		fmt.Println()

		fmt.Printf(
			"Local node is behind: local=%d remote=%d\n",
			localBefore.Height,
			remote.Height,
		)

		fmt.Println(
			"Requesting synchronization...",
		)

		if err := s.requestAndAdoptState(
			encoder,
			decoder,
			remote,
			false,
		); err != nil {
			return err
		}

	} else if remote.Height == localBefore.Height &&
		remote.LastHash != localBefore.LastHash {

		return fmt.Errorf(
			"fork detected at height %d: local=%s remote=%s",
			localBefore.Height,
			shortHash(localBefore.LastHash),
			shortHash(remote.LastHash),
		)
	}

	return s.requestPeers(
		encoder,
		decoder,
		remote,
	)
}

func (s *Server) requestPeers(
	encoder *json.Encoder,
	decoder *json.Decoder,
	remote HelloMessage,
) error {
	request := StateRequest{
		Type: MessageGetPeers,
	}

	if err := encoder.Encode(request); err != nil {
		return err
	}

	var response PeersResponse

	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if response.Type != MessagePeers {
		return fmt.Errorf(
			"unexpected peer discovery response: %s",
			response.Type,
		)
	}

	local := s.hello()

	if response.ChainID != remote.ChainID ||
		response.ChainID != local.ChainID {

		return fmt.Errorf(
			"peer discovery Chain ID mismatch",
		)
	}

	fmt.Println()

	seen := make(map[string]struct{})
	discovered := 0

	for _, peer := range response.Peers {
		if peer.NodeID == "" ||
			peer.Address == "" ||
			peer.NodeID == s.NodeID ||
			peer.NodeID == remote.NodeID {

			continue
		}

		if _, ok := seen[peer.NodeID]; ok {
			continue
		}

		seen[peer.NodeID] = struct{}{}

		if s.Peers.Has(peer.NodeID) {
			continue
		}

		if _, _, err := net.SplitHostPort(
			peer.Address,
		); err != nil {

			fmt.Println(
				"Ignoring malformed discovered peer:",
				peer.Address,
			)
			continue
		}

		discovered++

		fmt.Printf(
			"Discovered peer: %s at %s height=%d\n",
			peer.NodeID,
			peer.Address,
			peer.Height,
		)

		go s.connectDiscoveredPeer(
			peer,
		)
	}

	fmt.Printf(
		"Peer discovery: %d new peer(s).\n",
		discovered,
	)

	return nil
}

func (s *Server) connectDiscoveredPeer(
	peer PeerAdvertisement,
) {
	if peer.NodeID == "" ||
		peer.Address == "" ||
		s.Peers.Has(peer.NodeID) {

		return
	}

	s.dialMu.Lock()

	if s.dialing == nil {
		s.dialing = make(
			map[string]struct{},
		)
	}

	if _, exists := s.dialing[peer.NodeID]; exists {
		s.dialMu.Unlock()
		return
	}

	s.dialing[peer.NodeID] = struct{}{}
	s.dialMu.Unlock()

	defer func() {
		s.dialMu.Lock()
		delete(
			s.dialing,
			peer.NodeID,
		)
		s.dialMu.Unlock()
	}()

	fmt.Printf(
		"Connecting to discovered peer: %s at %s\n",
		peer.NodeID,
		peer.Address,
	)

	if err := s.connect(
		peer.Address,
		peer.NodeID,
	); err != nil {

		fmt.Printf(
			"Discovered peer connection failed: %s: %v\n",
			peer.NodeID,
			err,
		)
		return
	}

	fmt.Printf(
		"Discovered peer connected: %s at %s\n",
		peer.NodeID,
		peer.Address,
	)
}

func (s *Server) requestAndAdoptState(
	encoder *json.Encoder,
	decoder *json.Decoder,
	expectedRemote HelloMessage,
	allowDifferentGenesis bool,
) error {
	request := StateRequest{
		Type: MessageGetState,
	}

	if err := encoder.Encode(request); err != nil {
		return err
	}

	var response StateResponse

	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if response.Type != MessageState {
		return fmt.Errorf(
			"unexpected synchronization response: %s",
			response.Type,
		)
	}

	if response.ChainID != expectedRemote.ChainID {
		return fmt.Errorf(
			"state Chain ID mismatch: expected=%s received=%s",
			expectedRemote.ChainID,
			response.ChainID,
		)
	}

	if response.Blockchain == nil {
		return fmt.Errorf(
			"peer returned an empty blockchain",
		)
	}

	if len(response.Blockchain.Blocks) == 0 {
		return fmt.Errorf(
			"peer returned a blockchain without Genesis",
		)
	}

	genesisHash := response.Blockchain.Blocks[0].Hash

	calculatedChainID := MakeChainID(
		genesisHash,
	)

	if calculatedChainID != response.ChainID {
		return fmt.Errorf(
			"peer state has invalid Chain ID",
		)
	}

	remotePoS := &consensus.ProofOfStake{
		Validators: append(
			[]consensus.Validator(nil),
			response.Validators...,
		),
	}

	fmt.Println()
	fmt.Println(
		"Validating received blockchain...",
	)

	if !response.Blockchain.ValidateChain(
		remotePoS,
	) {
		return fmt.Errorf(
			"peer blockchain failed full validation",
		)
	}

	fmt.Println(
		"Remote blockchain validation: VALID",
	)

	localStatus := s.hello()

	if !allowDifferentGenesis {
		if response.ChainID != localStatus.ChainID {
			return fmt.Errorf(
				"refusing state from another Prism network",
			)
		}

		if !s.localChainIsPrefix(
			response.Blockchain,
		) {
			return fmt.Errorf(
				"remote chain does not extend local history",
			)
		}
	}

	remoteLast := response.Blockchain.Blocks[len(response.Blockchain.Blocks)-1]

	if remoteLast.Height < expectedRemote.Height {
		return fmt.Errorf(
			"peer returned stale state: advertised=%d received=%d",
			expectedRemote.Height,
			remoteLast.Height,
		)
	}

	if remoteLast.Height == expectedRemote.Height &&
		remoteLast.Hash != expectedRemote.LastHash {

		return fmt.Errorf(
			"peer state does not match advertised last hash",
		)
	}

	if response.ChainID == localStatus.ChainID &&
		remoteLast.Height <= localStatus.Height {

		return fmt.Errorf(
			"peer chain is not ahead of local chain",
		)
	}

	if err := storage.Save(
		s.DataDir,
		response.Blockchain,
		remotePoS,
		s.Wallets,
	); err != nil {
		return fmt.Errorf(
			"unable to persist synchronized state: %w",
			err,
		)
	}

	s.mu.Lock()
	s.Chain = response.Blockchain
	s.PoS = remotePoS
	s.mu.Unlock()

	updated := s.hello()

	fmt.Println()
	fmt.Println(
		"=== SYNCHRONIZATION COMPLETE ===",
	)

	fmt.Println(
		"Chain ID:",
		updated.ChainID,
	)

	fmt.Println(
		"Height:",
		updated.Height,
	)

	fmt.Println(
		"Genesis:",
		shortHash(updated.GenesisHash),
	)

	fmt.Println(
		"Last hash:",
		shortHash(updated.LastHash),
	)

	fmt.Println(
		"Chain valid: true",
	)

	return nil
}

func (s *Server) localChainIsPrefix(
	remote *blockchain.Blockchain,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Chain == nil ||
		remote == nil {

		return false
	}

	if len(s.Chain.Blocks) >
		len(remote.Blocks) {

		return false
	}

	for i := 0; i < len(s.Chain.Blocks); i++ {
		localBlock := s.Chain.Blocks[i]
		remoteBlock := remote.Blocks[i]

		if localBlock.Hash != remoteBlock.Hash {
			return false
		}
	}

	return true
}

func (s *Server) handleIncoming(
	conn net.Conn,
) {
	defer conn.Close()

	if err := conn.SetDeadline(
		time.Now().Add(10 * time.Second),
	); err != nil {
		fmt.Println(
			"P2P deadline error:",
			err,
		)
		return
	}

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var hello HelloMessage

	if err := decoder.Decode(&hello); err != nil {
		fmt.Println(
			"Invalid P2P hello:",
			err,
		)
		return
	}

	if err := validateHello(hello); err != nil {
		fmt.Println(
			"Handshake rejected:",
			err,
		)
		return
	}

	if hello.NodeID == s.NodeID {
		fmt.Println(
			"Handshake rejected: self connection",
		)
		return
	}

	peerAddress := reachablePeerAddress(
		hello.ListenAddr,
		conn.RemoteAddr(),
	)

	s.Peers.UpsertAt(
		hello,
		peerAddress,
	)

	fmt.Println()
	fmt.Println(
		"Incoming peer connected.",
	)

	s.printPeer(hello)

	if err := encoder.Encode(
		s.hello(),
	); err != nil {
		fmt.Println(
			"Unable to send handshake response:",
			err,
		)
		return
	}

	for {
		var request StateRequest

		if err := decoder.Decode(&request); err != nil {
			var netErr net.Error

			if errors.As(err, &netErr) &&
				netErr.Timeout() {

				return
			}

			return
		}

		switch request.Type {

		case MessageGetState:
			response := s.stateResponse()

			if err := encoder.Encode(
				response,
			); err != nil {
				fmt.Println(
					"Unable to send state snapshot:",
					err,
				)
				return
			}

			fmt.Println(
				"State snapshot sent to peer.",
			)

		case MessageGetPeers:
			local := s.hello()

			if hello.ChainID != local.ChainID {
				fmt.Println(
					"Peer discovery refused: different network.",
				)
				return
			}

			response := s.peersResponse(
				hello.NodeID,
			)

			if err := encoder.Encode(
				response,
			); err != nil {
				fmt.Println(
					"Unable to send peer list:",
					err,
				)
				return
			}

			fmt.Printf(
				"Peer list sent: %d peer(s).\n",
				len(response.Peers),
			)

		default:
			fmt.Println(
				"Unsupported P2P request:",
				request.Type,
			)
			return
		}
	}
}

func (s *Server) peersResponse(
	requesterNodeID string,
) PeersResponse {
	local := s.hello()

	return PeersResponse{
		Type:    MessagePeers,
		ChainID: local.ChainID,
		Peers: peerAdvertisements(
			s.Peers.List(),
			local.ChainID,
			s.NodeID,
			requesterNodeID,
		),
	}
}

func (s *Server) stateResponse() StateResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	validators := append(
		[]consensus.Validator(nil),
		s.PoS.Validators...,
	)

	genesisHash := s.Chain.Blocks[0].Hash

	return StateResponse{
		Type:       MessageState,
		ChainID:    MakeChainID(genesisHash),
		Blockchain: s.Chain,
		Validators: validators,
	}
}

func (s *Server) hello() HelloMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Chain == nil ||
		len(s.Chain.Blocks) == 0 {

		return HelloMessage{
			Type:       MessageHello,
			Version:    ProtocolVersion,
			NodeID:     s.NodeID,
			ListenAddr: s.ListenAddr,
		}
	}

	genesis := s.Chain.Blocks[0]

	last := s.Chain.Blocks[len(s.Chain.Blocks)-1]

	return HelloMessage{
		Type:        MessageHello,
		Version:     ProtocolVersion,
		NodeID:      s.NodeID,
		ListenAddr:  s.ListenAddr,
		ChainID:     MakeChainID(genesis.Hash),
		GenesisHash: genesis.Hash,
		Height:      last.Height,
		LastHash:    last.Hash,
	}
}

func validateHello(
	message HelloMessage,
) error {
	if message.Type != MessageHello {
		return fmt.Errorf(
			"unexpected message type: %s",
			message.Type,
		)
	}

	if message.Version != ProtocolVersion {
		return fmt.Errorf(
			"protocol mismatch: local=%s remote=%s",
			ProtocolVersion,
			message.Version,
		)
	}

	if message.NodeID == "" {
		return fmt.Errorf(
			"peer node ID cannot be empty",
		)
	}

	if message.ListenAddr == "" {
		return fmt.Errorf(
			"peer listen address cannot be empty",
		)
	}

	if message.ChainID == "" {
		return fmt.Errorf(
			"peer Chain ID cannot be empty",
		)
	}

	if message.GenesisHash == "" {
		return fmt.Errorf(
			"peer Genesis hash cannot be empty",
		)
	}

	if message.LastHash == "" {
		return fmt.Errorf(
			"peer last hash cannot be empty",
		)
	}

	expectedChainID := MakeChainID(
		message.GenesisHash,
	)

	if message.ChainID != expectedChainID {
		return fmt.Errorf(
			"peer Chain ID does not match its Genesis",
		)
	}

	return nil
}

func (s *Server) printPeer(
	peer HelloMessage,
) {
	local := s.hello()

	fmt.Println(
		"Peer ID:",
		peer.NodeID,
	)

	fmt.Println(
		"Peer address:",
		peer.ListenAddr,
	)

	fmt.Println(
		"Peer Chain ID:",
		peer.ChainID,
	)

	fmt.Println(
		"Peer height:",
		peer.Height,
	)

	fmt.Println(
		"Peer Genesis:",
		shortHash(peer.GenesisHash),
	)

	fmt.Println(
		"Peer last hash:",
		shortHash(peer.LastHash),
	)

	if peer.ChainID != local.ChainID {
		fmt.Println(
			"Chain state: DIFFERENT NETWORK",
		)
		return
	}

	if peer.Height == local.Height &&
		peer.LastHash == local.LastHash {

		fmt.Println(
			"Chain state: MATCH",
		)
		return
	}

	if peer.Height > local.Height {
		fmt.Println(
			"Chain state: LOCAL NODE BEHIND",
		)
		return
	}

	if peer.Height < local.Height {
		fmt.Println(
			"Chain state: REMOTE NODE BEHIND",
		)
		return
	}

	fmt.Println(
		"Chain state: FORK",
	)
}

func reachablePeerAddress(
	listenAddr string,
	remoteAddr net.Addr,
) string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return listenAddr
	}

	if host != "" &&
		host != "0.0.0.0" &&
		host != "::" {

		return listenAddr
	}

	if remoteAddr == nil {
		return listenAddr
	}

	remoteHost, _, err := net.SplitHostPort(
		remoteAddr.String(),
	)
	if err != nil {
		return listenAddr
	}

	return net.JoinHostPort(
		remoteHost,
		port,
	)
}

func shortHash(
	hash string,
) string {
	if len(hash) <= 20 {
		return hash
	}

	return hash[:12] +
		"..." +
		hash[len(hash)-8:]
}
