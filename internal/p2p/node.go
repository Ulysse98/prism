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

const ProtocolVersion = "0.16"

const (
	MessageHello    = "hello"
	MessageGetState = "get_state"
	MessageState    = "state"
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

type Server struct {
	NodeID     string
	ListenAddr string
	DataDir    string

	Chain   *blockchain.Blockchain
	PoS     *consensus.ProofOfStake
	Wallets map[string]*wallet.Wallet

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
	}

	fmt.Println()
	fmt.Println(
		"P2P node running. Press Ctrl+C to stop.",
	)

	return <-errCh
}

func (s *Server) Connect(address string) error {
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

		return s.requestAndAdoptState(
			encoder,
			decoder,
			remote,
			true,
		)
	}

	if remote.Height > localBefore.Height {
		fmt.Println()

		fmt.Printf(
			"Local node is behind: local=%d remote=%d\n",
			localBefore.Height,
			remote.Height,
		)

		fmt.Println(
			"Requesting synchronization...",
		)

		return s.requestAndAdoptState(
			encoder,
			decoder,
			remote,
			false,
		)
	}

	if remote.Height == localBefore.Height &&
		remote.LastHash != localBefore.LastHash {

		return fmt.Errorf(
			"fork detected at height %d: local=%s remote=%s",
			localBefore.Height,
			shortHash(localBefore.LastHash),
			shortHash(remote.LastHash),
		)
	}

	return nil
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

	var request StateRequest

	if err := decoder.Decode(&request); err != nil {
		var netErr net.Error

		if errors.As(err, &netErr) &&
			netErr.Timeout() {

			return
		}

		return
	}

	if request.Type != MessageGetState {
		fmt.Println(
			"Unsupported P2P request:",
			request.Type,
		)
		return
	}

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
