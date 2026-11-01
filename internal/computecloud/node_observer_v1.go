//go:build linux

package computecloud

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

const maxNodeStatusBodyV1 = 8192

var (
	ErrCloudNodeUnavailableV1 = errors.New("prism node unavailable")
	ErrCloudNodeMismatchV1    = errors.New("prism node identity mismatch")
	ErrCloudNodeUnhealthyV1   = errors.New("prism node status invalid or unhealthy")
	validNodeChainIDV1        = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	validNodeVersionV1        = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,64}$`)
)

// CloudNodeObserverV1 only reads /api/v1/status from 127.0.0.1.
// It never forwards SaaS Authorization headers, writes to Prism, or inspects wallets.
type CloudNodeObserverV1 struct {
	endpoint        string
	expectedChainID string
	expectedGenesis string
	client          *http.Client
}

// NewCloudNodeObserverV1 refuses a network target unless its identity is pinned.
// There is deliberately no arbitrary URL or hostname configuration.
func NewCloudNodeObserverV1(port int, chainID, genesisHash string) (*CloudNodeObserverV1, error) {
	genesisBytes, err := hex.DecodeString(genesisHash)
	if port < 1 || port > 65535 ||
		!validNodeChainIDV1.MatchString(chainID) ||
		err != nil || len(genesisBytes) != 32 || hex.EncodeToString(genesisBytes) != genesisHash {
		return nil, errors.New("observer requires valid loopback port, chain ID, and lowercase 32-byte genesis hash")
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	transport := &http.Transport{
		Proxy:             nil, // Never use HTTP_PROXY for a local Prism node.
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, target string) (net.Conn, error) {
			if network != "tcp" || target != address {
				return nil, ErrCloudNodeUnavailableV1
			}
			dialer := &net.Dialer{Timeout: time.Second}
			return dialer.DialContext(ctx, "tcp4", address)
		},
	}
	return &CloudNodeObserverV1{
		endpoint:        "http://" + address + "/api/v1/status",
		expectedChainID: chainID,
		expectedGenesis: genesisHash,
		client: &http.Client{
			Timeout:       1500 * time.Millisecond,
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// CloudNodeStatusV1 is intentionally an allowlisted subset of Prism's status.
// A healthy node does not imply a working GPU or an authorized job dispatcher.
type CloudNodeStatusV1 struct {
	Mode            string `json:"mode"`
	NodeVersion     string `json:"node_version"`
	ChainID         string `json:"chain_id"`
	Height          uint64 `json:"height"`
	ChainValid      bool   `json:"chain_valid"`
	ComputeEnabled  bool   `json:"compute_enabled"`
	ProofsAvailable bool   `json:"proofs_available"`
}

func (n *CloudNodeObserverV1) ReadStatus(ctx context.Context) (CloudNodeStatusV1, error) {
	if n == nil {
		return CloudNodeStatusV1{}, ErrCloudNodeUnavailableV1
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.endpoint, nil)
	if err != nil {
		return CloudNodeStatusV1{}, ErrCloudNodeUnavailableV1
	}
	req.Header.Set("Accept", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return CloudNodeStatusV1{}, ErrCloudNodeUnavailableV1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CloudNodeStatusV1{}, ErrCloudNodeUnavailableV1
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxNodeStatusBodyV1+1))
	if err != nil || len(data) > maxNodeStatusBodyV1 {
		return CloudNodeStatusV1{}, ErrCloudNodeUnhealthyV1
	}
	var raw struct {
		Version     string `json:"version"`
		ChainID     string `json:"chainId"`
		GenesisHash string `json:"genesisHash"`
		Height      uint64 `json:"height"`
		ChainValid  *bool  `json:"chainValid"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return CloudNodeStatusV1{}, ErrCloudNodeUnhealthyV1
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return CloudNodeStatusV1{}, ErrCloudNodeUnhealthyV1
	}
	if raw.ChainID != n.expectedChainID || raw.GenesisHash != n.expectedGenesis {
		return CloudNodeStatusV1{}, ErrCloudNodeMismatchV1
	}
	if raw.ChainValid == nil || !*raw.ChainValid || !validNodeVersionV1.MatchString(raw.Version) {
		return CloudNodeStatusV1{}, ErrCloudNodeUnhealthyV1
	}
	return CloudNodeStatusV1{
		Mode: "read_only", NodeVersion: raw.Version,
		ChainID: raw.ChainID, Height: raw.Height, ChainValid: true,
		ComputeEnabled: false, ProofsAvailable: false,
	}, nil
}

func (n *CloudNodeObserverV1) String() string {
	if n == nil {
		return "disabled"
	}
	return fmt.Sprintf("loopback_only:%s", n.expectedChainID)
}
