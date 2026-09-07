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
	"prism/internal/mempool"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

const ProtocolVersion = "0.19"

const (
	MessageHello          = "hello"
	MessageGetState       = "get_state"
	MessageState          = "state"
	MessageGetPeers       = "get_peers"
	MessagePeers          = "peers"
	MessageTransaction    = "transaction"
	MessageTransactionAck = "transaction_ack"
	MessageBlock          = "block"
	MessageBlockAck       = "block_ack"
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

type TransactionMessage struct {
	Type        string                  `json:"type"`
	ChainID     string                  `json:"chain_id"`
	Transaction transaction.Transaction `json:"transaction"`
}

type TransactionAck struct {
	Type          string `json:"type"`
	TransactionID string `json:"transaction_id"`
	Accepted      bool   `json:"accepted"`
	Error         string `json:"error,omitempty"`
}

type BlockMessage struct {
	Type    string           `json:"type"`
	ChainID string           `json:"chain_id"`
	Block   blockchain.Block `json:"block"`
}

type BlockAck struct {
	Type      string `json:"type"`
	BlockHash string `json:"block_hash"`
	Accepted  bool   `json:"accepted"`
	Error     string `json:"error,omitempty"`
}

type Server struct {
	NodeID     string
	ListenAddr string
	DataDir    string

	Chain   *blockchain.Blockchain
	PoS     *consensus.ProofOfStake
	Wallets map[string]*wallet.Wallet
	Peers   *PeerBook
	Pool    *mempool.Mempool

	poolMu sync.Mutex

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
		Pool:       mempool.New(),
		dialing:    make(map[string]struct{}),
	}
}

func MakeNodeID(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:8])
}

func MakeChainID(
	genesisHash string,
) string {
	return blockchain.MakeChainID(
		genesisHash,
	)
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

func (s *Server) SendTransaction(
	address string,
	tx transaction.Transaction,
) error {
	if err := transaction.ValidateSigned(tx); err != nil {
		return fmt.Errorf(
			"invalid outgoing transaction: %w",
			err,
		)
	}

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

	local := s.hello()

	if err := encoder.Encode(local); err != nil {
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
			"refusing self transaction submission",
		)
	}

	if remote.ChainID != local.ChainID {
		return fmt.Errorf(
			"transaction peer network mismatch: local=%s remote=%s",
			local.ChainID,
			remote.ChainID,
		)
	}

	message := TransactionMessage{
		Type:        MessageTransaction,
		ChainID:     local.ChainID,
		Transaction: tx,
	}

	if err := encoder.Encode(message); err != nil {
		return err
	}

	var ack TransactionAck

	if err := decoder.Decode(&ack); err != nil {
		return err
	}

	if ack.Type != MessageTransactionAck {
		return fmt.Errorf(
			"unexpected transaction response: %s",
			ack.Type,
		)
	}

	if ack.TransactionID != tx.ID {
		return fmt.Errorf(
			"transaction acknowledgement ID mismatch",
		)
	}

	if !ack.Accepted {
		if ack.Error == "" {
			ack.Error = "transaction rejected"
		}

		return fmt.Errorf(
			"%s",
			ack.Error,
		)
	}

	return nil
}

func (s *Server) SendBlock(
	address string,
	block blockchain.Block,
) error {
	if block.Hash == "" {
		return fmt.Errorf(
			"outgoing block hash cannot be empty",
		)
	}

	if blockchain.CalculateHash(block) != block.Hash {
		return fmt.Errorf(
			"invalid outgoing block hash",
		)
	}

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

	local := s.hello()

	if err := encoder.Encode(local); err != nil {
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
			"refusing self block submission",
		)
	}

	if remote.ChainID != local.ChainID {
		return fmt.Errorf(
			"block peer network mismatch: local=%s remote=%s",
			local.ChainID,
			remote.ChainID,
		)
	}

	message := BlockMessage{
		Type:    MessageBlock,
		ChainID: local.ChainID,
		Block:   block,
	}

	if err := encoder.Encode(message); err != nil {
		return err
	}

	var ack BlockAck

	if err := decoder.Decode(&ack); err != nil {
		return err
	}

	if ack.Type != MessageBlockAck {
		return fmt.Errorf(
			"unexpected block response: %s",
			ack.Type,
		)
	}

	if ack.BlockHash != block.Hash {
		return fmt.Errorf(
			"block acknowledgement hash mismatch",
		)
	}

	if !ack.Accepted {
		if ack.Error == "" {
			ack.Error = "block rejected"
		}

		return fmt.Errorf(
			"%s",
			ack.Error,
		)
	}

	return nil
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
	fmt.Println("Incoming peer connected.")
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
		var raw json.RawMessage

		if err := decoder.Decode(&raw); err != nil {
			var netErr net.Error

			if errors.As(err, &netErr) &&
				netErr.Timeout() {

				return
			}

			return
		}

		var request StateRequest

		if err := json.Unmarshal(
			raw,
			&request,
		); err != nil {
			fmt.Println(
				"Invalid P2P request:",
				err,
			)
			return
		}

		switch request.Type {

		case MessageGetState:
			response := s.stateResponse()

			if err := encoder.Encode(response); err != nil {
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

			if err := encoder.Encode(response); err != nil {
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

		case MessageTransaction:
			var message TransactionMessage

			if err := json.Unmarshal(
				raw,
				&message,
			); err != nil {
				_ = encoder.Encode(
					TransactionAck{
						Type:     MessageTransactionAck,
						Accepted: false,
						Error:    "invalid transaction message",
					},
				)
				continue
			}

			local := s.hello()

			ack := TransactionAck{
				Type:          MessageTransactionAck,
				TransactionID: message.Transaction.ID,
			}

			if hello.ChainID != local.ChainID ||
				message.ChainID != local.ChainID {

				ack.Error = "transaction network mismatch"

				if err := encoder.Encode(ack); err != nil {
					return
				}

				fmt.Println(
					"Transaction rejected: network mismatch",
				)

				continue
			}

			if err := s.acceptTransaction(
				message.Transaction,
			); err != nil {

				if s.HasTransaction(
					message.Transaction.ID,
				) {
					ack.Accepted = true

					if err := encoder.Encode(ack); err != nil {
						return
					}

					fmt.Printf(
						"Transaction already known: %s mempool=%d\n",
						message.Transaction.ID,
						s.MempoolCount(),
					)

					continue
				}

				ack.Error = err.Error()

				if err := encoder.Encode(ack); err != nil {
					return
				}

				fmt.Printf(
					"Transaction rejected: %s: %v\n",
					message.Transaction.ID,
					err,
				)

				continue
			}

			ack.Accepted = true

			if err := encoder.Encode(ack); err != nil {
				return
			}

			fmt.Printf(
				"Transaction accepted: %s mempool=%d\n",
				message.Transaction.ID,
				s.MempoolCount(),
			)

			go s.broadcastTransaction(
				message.Transaction,
				hello.NodeID,
			)

		case MessageBlock:
			var message BlockMessage

			if err := json.Unmarshal(
				raw,
				&message,
			); err != nil {
				_ = encoder.Encode(
					BlockAck{
						Type:     MessageBlockAck,
						Accepted: false,
						Error:    "invalid block message",
					},
				)
				continue
			}

			local := s.hello()

			ack := BlockAck{
				Type:      MessageBlockAck,
				BlockHash: message.Block.Hash,
			}

			if hello.ChainID != local.ChainID ||
				message.ChainID != local.ChainID {

				ack.Error = "block network mismatch"

				if err := encoder.Encode(ack); err != nil {
					return
				}

				fmt.Println(
					"Block rejected: network mismatch",
				)

				continue
			}

			appended, err := s.acceptBlock(
				message.Block,
			)

			if err != nil {
				ack.Error = err.Error()

				if err := encoder.Encode(ack); err != nil {
					return
				}

				fmt.Printf(
					"Block rejected: height=%d hash=%s: %v\n",
					message.Block.Height,
					shortHash(message.Block.Hash),
					err,
				)

				continue
			}

			ack.Accepted = true

			if err := encoder.Encode(ack); err != nil {
				return
			}

			if !appended {
				fmt.Printf(
					"Block already known: height=%d hash=%s\n",
					message.Block.Height,
					shortHash(message.Block.Hash),
				)

				continue
			}

			fmt.Printf(
				"Block accepted: height=%d hash=%s\n",
				message.Block.Height,
				shortHash(message.Block.Hash),
			)

			go s.broadcastBlock(
				message.Block,
				hello.NodeID,
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

func (s *Server) acceptTransaction(
	tx transaction.Transaction,
) error {
	s.mu.RLock()
	chain := s.Chain
	s.mu.RUnlock()

	if chain == nil {
		return fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	if s.Pool == nil {
		s.Pool = mempool.New()
	}

	return s.Pool.Add(
		tx,
		chain,
	)
}

func (s *Server) HasTransaction(
	transactionID string,
) bool {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	if s.Pool == nil {
		return false
	}

	return s.Pool.Has(
		transactionID,
	)
}

func (s *Server) broadcastTransaction(
	tx transaction.Transaction,
	excludeNodeID string,
) {
	local := s.hello()

	for _, peer := range s.Peers.List() {
		if peer.NodeID == "" ||
			peer.Address == "" ||
			peer.NodeID == s.NodeID ||
			peer.NodeID == excludeNodeID ||
			peer.ChainID != local.ChainID {

			continue
		}

		currentPeer := peer

		go func() {
			if err := s.SendTransaction(
				currentPeer.Address,
				tx,
			); err != nil {

				fmt.Printf(
					"Transaction broadcast failed: %s -> %s: %v\n",
					tx.ID,
					currentPeer.NodeID,
					err,
				)

				return
			}

			fmt.Printf(
				"Transaction broadcast accepted: %s -> %s\n",
				tx.ID,
				currentPeer.NodeID,
			)
		}()
	}
}

func (s *Server) acceptBlock(
	block blockchain.Block,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Chain == nil {
		return false, fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	if s.PoS == nil {
		return false, fmt.Errorf(
			"proof of stake engine cannot be nil",
		)
	}

	if len(s.Chain.Blocks) == 0 {
		return false, fmt.Errorf(
			"blockchain has no genesis block",
		)
	}

	localTip := s.Chain.Blocks[len(s.Chain.Blocks)-1]

	if block.Height <= localTip.Height {
		if block.Height < uint64(len(s.Chain.Blocks)) {
			existing := s.Chain.Blocks[int(block.Height)]

			if existing.Hash == block.Hash {
				return false, nil
			}
		}

		return false, fmt.Errorf(
			"conflicting or stale block at height %d",
			block.Height,
		)
	}

	oldLength := len(s.Chain.Blocks)

	if err := s.Chain.AppendValidatedBlock(
		block,
		s.PoS,
	); err != nil {
		return false, err
	}

	if err := storage.Save(
		s.DataDir,
		s.Chain,
		s.PoS,
		s.Wallets,
	); err != nil {

		s.Chain.Blocks =
			s.Chain.Blocks[:oldLength]

		return false, fmt.Errorf(
			"unable to persist received block: %w",
			err,
		)
	}

	s.poolMu.Lock()

	if s.Pool != nil {
		s.Pool.RemoveCommitted(
			block.Transactions,
		)
	}

	s.poolMu.Unlock()

	return true, nil
}

func (s *Server) broadcastBlock(
	block blockchain.Block,
	excludeNodeID string,
) {
	local := s.hello()

	for _, peer := range s.Peers.List() {
		if peer.NodeID == "" ||
			peer.Address == "" ||
			peer.NodeID == s.NodeID ||
			peer.NodeID == excludeNodeID ||
			peer.ChainID != local.ChainID {

			continue
		}

		currentPeer := peer

		go func() {
			if err := s.SendBlock(
				currentPeer.Address,
				block,
			); err != nil {

				fmt.Printf(
					"Block broadcast failed: height=%d -> %s: %v\n",
					block.Height,
					currentPeer.NodeID,
					err,
				)

				return
			}

			fmt.Printf(
				"Block broadcast accepted: height=%d -> %s\n",
				block.Height,
				currentPeer.NodeID,
			)
		}()
	}
}

func (s *Server) MempoolCount() int {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	if s.Pool == nil {
		return 0
	}

	return s.Pool.Count()
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
