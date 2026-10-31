//go:build linux

package computecloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

const testNodeChainID = "prism-isolated-test"
const testNodeGenesisHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func makeNodeServer(t *testing.T, handler http.HandlerFunc) (int, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	port, err := strconv.Atoi(strings.TrimPrefix(server.Listener.Addr().String(), "127.0.0.1:"))
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return port, server.Close
}

func validNodeStatusJSON() string {
	return fmt.Sprintf(`{"version":"0.63.0","chainId":%q,"genesisHash":%q,"height":42,"chainValid":true,"walletChainId":"do-not-disclose","secret":"do-not-disclose"}`,
		testNodeChainID, testNodeGenesisHash)
}

func TestCloudObserverReadOnlyAndFiltered(t *testing.T) {
	var calls atomic.Int64
	port, closeServer := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/api/v1/status" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected upstream request: %s %s auth=%q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, validNodeStatusJSON())
	})
	defer closeServer()
	observer, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
	if err != nil {
		t.Fatal(err)
	}
	status, err := observer.ReadStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Height != 42 || status.ChainID != testNodeChainID || !status.ChainValid || status.ComputeEnabled || status.ProofsAvailable {
		t.Fatalf("unsafe observed node status: %+v", status)
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls: %d", calls.Load())
	}
}

func TestCloudObserverRefusesInvalidConfiguration(t *testing.T) {
	for _, tt := range []struct {
		port             int
		chainID, genesis string
	}{
		{0, testNodeChainID, testNodeGenesisHash},
		{65536, testNodeChainID, testNodeGenesisHash},
		{80, "http://outside.example", testNodeGenesisHash},
		{80, testNodeChainID, "short"},
		{80, testNodeChainID, strings.ToUpper(testNodeGenesisHash[:10]) + testNodeGenesisHash[10:]},
	} {
		if _, err := NewCloudNodeObserverV1(tt.port, tt.chainID, tt.genesis); err == nil {
			t.Fatalf("unsafe observer configuration accepted: %+v", tt)
		}
	}
}

func TestCloudObserverRejectsBadNodeResponses(t *testing.T) {
	tests := []struct {
		name, body string
		statusCode int
		want       error
	}{
		{"wrong_chain", strings.Replace(validNodeStatusJSON(), testNodeChainID, "prism-other-network", 1), 200, ErrCloudNodeMismatchV1},
		{"wrong_genesis", strings.Replace(validNodeStatusJSON(), testNodeGenesisHash, strings.Repeat("b", 64), 1), 200, ErrCloudNodeMismatchV1},
		{"invalid_chain", strings.Replace(validNodeStatusJSON(), `"chainValid":true`, `"chainValid":false`, 1), 200, ErrCloudNodeUnhealthyV1},
		{"missing_valid", strings.Replace(validNodeStatusJSON(), `,"chainValid":true`, "", 1), 200, ErrCloudNodeUnhealthyV1},
		{"invalid_json", "not json", 200, ErrCloudNodeUnhealthyV1},
		{"oversized", strings.Repeat("X", maxNodeStatusBodyV1+2), 200, ErrCloudNodeUnhealthyV1},
		{"trailing_json", validNodeStatusJSON() + "{}", 200, ErrCloudNodeUnhealthyV1},
		{"redirect", "", 302, ErrCloudNodeUnavailableV1},
		{"server_error", "", 500, ErrCloudNodeUnavailableV1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, closeServer := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode == 302 {
					w.Header().Set("Location", "http://example.com/steal")
				}
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.body)
			})
			defer closeServer()
			o, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
			if err != nil {
				t.Fatal(err)
			}
			_, err = o.ReadStatus(context.Background())
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCloudObserverNoConnectionWhenCancelled(t *testing.T) {
	port, closeServer := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validNodeStatusJSON())
	})
	defer closeServer()
	o, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := o.ReadStatus(ctx); !errors.Is(err, ErrCloudNodeUnavailableV1) {
		t.Fatalf("cancelled request: %v", err)
	}
}

func TestCloudObserverAlwaysLoopback(t *testing.T) {
	port, closeServer := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, validNodeStatusJSON()) })
	defer closeServer()
	o, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
	if err != nil {
		t.Fatal(err)
	}
	dialer, ok := o.client.Transport.(*http.Transport)
	if !ok || dialer.Proxy != nil {
		t.Fatal("unsafe transport")
	}
	_, err = dialer.DialContext(context.Background(), "tcp", net.JoinHostPort("example.com", strconv.Itoa(port)))
	if !errors.Is(err, ErrCloudNodeUnavailableV1) {
		t.Fatalf("non-loopback dial accepted: %v", err)
	}
}
