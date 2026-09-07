package p2p

import (
	"net"
	"testing"
)

func TestReachablePeerAddressReplacesWildcardHost(t *testing.T) {
	remote := &net.TCPAddr{
		IP:   net.ParseIP("172.20.0.3"),
		Port: 54321,
	}

	got := reachablePeerAddress(
		"0.0.0.0:7003",
		remote,
	)

	if got != "172.20.0.3:7003" {
		t.Fatalf(
			"expected 172.20.0.3:7003, got %s",
			got,
		)
	}
}

func TestReachablePeerAddressPreservesAdvertisedHost(t *testing.T) {
	remote := &net.TCPAddr{
		IP:   net.ParseIP("172.20.0.3"),
		Port: 54321,
	}

	got := reachablePeerAddress(
		"prism-node-3:7003",
		remote,
	)

	if got != "prism-node-3:7003" {
		t.Fatalf(
			"expected advertised address to survive, got %s",
			got,
		)
	}
}
