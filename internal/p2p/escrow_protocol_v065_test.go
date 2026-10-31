package p2p

import (
	"strings"
	"testing"

	"prism/internal/blockchain"
)

func TestEscrowV065ProtocolIsolation(t *testing.T) {
	if ProtocolVersion != "0.65-escrow-lab" {
		t.Fatalf("unexpected protocol: %s", ProtocolVersion)
	}

	genesis := "prism-v065-isolated-genesis"
	hello := HelloMessage{
		Type:        MessageHello,
		Version:     ProtocolVersion,
		NodeID:      "escrow-node-b",
		ListenAddr:  "127.0.0.1:17066",
		GenesisHash: genesis,
		ChainID:     blockchain.MakeChainID(genesis),
		LastHash:    genesis,
	}

	if err := validateHello(hello); err != nil {
		t.Fatalf("valid escrow peer rejected: %v", err)
	}

	hello.Version = "0.36"

	err := validateHello(hello)
	if err == nil || !strings.Contains(
		err.Error(), "protocol mismatch",
	) {
		t.Fatalf("legacy peer not rejected: %v", err)
	}
}
