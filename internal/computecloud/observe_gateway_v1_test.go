//go:build linux

package computecloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func observeRequest(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestCloudObserveGatewayAuthAndSafeCapability(t *testing.T) {
	g, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "private", "jobs.json"))
	defer store.Close()
	h := &CloudObserveGatewayV1{Gateway: g}
	if r := observeRequest(h, "GET", "/cloud/v1/capabilities", ""); r.Code != 401 {
		t.Fatalf("unauthed capability %d", r.Code)
	}
	if r := observeRequest(h, "GET", "/cloud/v1/node", "invalid"); r.Code != 401 {
		t.Fatalf("invalid token %d", r.Code)
	}
	if r := observeRequest(h, "POST", "/cloud/v1/node", testCloudTokenA); r.Code != 405 {
		t.Fatalf("bad method %d", r.Code)
	}
	if r := observeRequest(h, "GET", "/cloud/v1/node?host=evil", testCloudTokenA); r.Code != 400 {
		t.Fatalf("query accepted %d", r.Code)
	}
	if r := observeRequest(h, "GET", "/cloud/v1/node", testCloudTokenA); r.Code != 503 {
		t.Fatalf("disabled node %d", r.Code)
	}
	cap := observeRequest(h, "GET", "/cloud/v1/capabilities", testCloudTokenA)
	if cap.Code != 200 {
		t.Fatalf("capability %d", cap.Code)
	}
	var result struct {
		Mode             string `json:"mode"`
		MaxBatch         uint64 `json:"max_batch"`
		ComputeEnabled   bool   `json:"compute_enabled"`
		ProofsAvailable  bool   `json:"proofs_available"`
		NodeProbeEnabled bool   `json:"node_probe_enabled"`
	}
	if err := json.Unmarshal(cap.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "simulation" || result.MaxBatch != MaxBatchSizeV1 || result.ComputeEnabled || result.ProofsAvailable || result.NodeProbeEnabled {
		t.Fatalf("unsafe capability: %+v", result)
	}
	if r := observeRequest(h, "GET", "/cloud/v1/usage", testCloudTokenA); r.Code != 200 {
		t.Fatalf("v1 route regressed: %d", r.Code)
	}
}

func TestCloudObserveGatewayFiltersAndNeverForwardsAuth(t *testing.T) {
	g, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "private", "jobs.json"))
	defer store.Close()
	var calls atomic.Int64
	port, closeServer := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("forwarded bearer token to Prism node")
		}
		fmt.Fprint(w, validNodeStatusJSON())
	})
	defer closeServer()
	o, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
	if err != nil {
		t.Fatal(err)
	}
	h := &CloudObserveGatewayV1{Gateway: g, Observer: o}
	if r := observeRequest(h, "GET", "/cloud/v1/node", ""); r.Code != 401 {
		t.Fatalf("unauthenticated node returned %d", r.Code)
	}
	if r := observeRequest(h, "POST", "/cloud/v1/node", testCloudTokenA); r.Code != 405 {
		t.Fatalf("node mutation returned %d", r.Code)
	}
	if calls.Load() != 0 {
		t.Fatal("unauthorized request reached upstream node")
	}
	resp := observeRequest(h, "GET", "/cloud/v1/node", testCloudTokenA)
	if resp.Code != 200 {
		t.Fatalf("node status %d: %s", resp.Code, resp.Body.String())
	}
	for _, forbidden := range []string{"walletChainId", "secret", "do-not-disclose", "genesisHash"} {
		if strings.Contains(resp.Body.String(), forbidden) {
			t.Fatalf("sensitive field %q leaked", forbidden)
		}
	}
	if !strings.Contains(resp.Body.String(), `"compute_enabled":false`) {
		t.Fatal("node falsely claims GPU execution")
	}
	if r := observeRequest(h, "GET", "/cloud/v1/node", testCloudTokenB); r.Code != 200 {
		t.Fatalf("second valid tenant blocked: %d", r.Code)
	}
	if r := observeRequest(h, "GET", "/cloud/v1/jobs", testCloudTokenA); r.Code != 405 {
		t.Fatalf("existing method behavior changed: %d", r.Code)
	}
}
