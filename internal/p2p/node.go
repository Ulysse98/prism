package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const ProtocolVersion = "0.14"

type HelloMessage struct {
	Type       string `json:"type"`
	Version    string `json:"version"`
	NodeID     string `json:"node_id"`
	ListenAddr string `json:"listen_addr"`
	Height     uint64 `json:"height"`
	LastHash   string `json:"last_hash"`
}

type Server struct {
	NodeID     string
	ListenAddr string
	Height     uint64
	LastHash   string
}

func NewServer(
	nodeID string,
	listenAddr string,
	height uint64,
	lastHash string,
) *Server {
	return &Server{
		NodeID:     nodeID,
		ListenAddr: listenAddr,
		Height:     height,
		LastHash:   lastHash,
	}
}

func MakeNodeID(seed string) string {
	hash := sha256.Sum256([]byte(seed))

	return hex.EncodeToString(hash[:8])
}

func (s *Server) Run(peer string) error {
	listener, err := net.Listen(
		"tcp",
		s.ListenAddr,
	)
	if err != nil {
		return err
	}

	defer listener.Close()

	fmt.Println("=== P2P NODE ===")
	fmt.Println("Node ID:", s.NodeID)
	fmt.Println("Listening:", s.ListenAddr)
	fmt.Println("Protocol:", ProtocolVersion)
	fmt.Println("Height:", s.Height)
	fmt.Println("Last hash:", shortHash(s.LastHash))

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
		fmt.Println("Connecting to peer:", peer)

		if err := s.Connect(peer); err != nil {
			return fmt.Errorf(
				"peer connection failed: %w",
				err,
			)
		}
	}

	fmt.Println()
	fmt.Println("P2P node running. Press Ctrl+C to stop.")

	return <-errCh
}

func (s *Server) Connect(
	address string,
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
		time.Now().Add(5 * time.Second),
	); err != nil {
		return err
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(
		s.hello(),
	); err != nil {
		return err
	}

	var response HelloMessage

	if err := decoder.Decode(
		&response,
	); err != nil {
		return err
	}

	if err := validateHello(response); err != nil {
		return err
	}

	fmt.Println("Handshake accepted.")

	s.printPeer(response)

	return nil
}

func (s *Server) handleIncoming(
	conn net.Conn,
) {
	defer conn.Close()

	if err := conn.SetDeadline(
		time.Now().Add(5 * time.Second),
	); err != nil {
		fmt.Println("P2P deadline error:", err)
		return
	}

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var hello HelloMessage

	if err := decoder.Decode(
		&hello,
	); err != nil {
		fmt.Println("Invalid P2P message:", err)
		return
	}

	if err := validateHello(hello); err != nil {
		fmt.Println("Handshake rejected:", err)
		return
	}

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
	}
}

func (s *Server) hello() HelloMessage {
	return HelloMessage{
		Type:       "hello",
		Version:    ProtocolVersion,
		NodeID:     s.NodeID,
		ListenAddr: s.ListenAddr,
		Height:     s.Height,
		LastHash:   s.LastHash,
	}
}

func validateHello(
	message HelloMessage,
) error {
	if message.Type != "hello" {
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

	return nil
}

func (s *Server) printPeer(
	peer HelloMessage,
) {
	fmt.Println("Peer ID:", peer.NodeID)
	fmt.Println("Peer address:", peer.ListenAddr)
	fmt.Println("Peer height:", peer.Height)
	fmt.Println(
		"Peer last hash:",
		shortHash(peer.LastHash),
	)

	if peer.Height == s.Height &&
		peer.LastHash == s.LastHash {

		fmt.Println("Chain state: MATCH")
		return
	}

	fmt.Println("Chain state: DIFFERENT")
	fmt.Println(
		"Synchronization will arrive in Prism v0.15.",
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
