//go:build linux

package computecloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const validPlanBodyV1 = `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`

func requestPlanV1(h http.Handler, method, path, token, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, req)
	return r
}

func TestCloudPlanGatewayPreflightAndNoSideEffects(t *testing.T) {
	base, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "private", "jobs.json"))
	defer store.Close()
	var calls atomic.Int64
	port, closeNode := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/status" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "" || r.ContentLength > 0 {
			t.Errorf("unsafe upstream call: %s %s; auth=%q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, validNodeStatusJSON())
	})
	defer closeNode()
	observer, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
	if err != nil {
		t.Fatal(err)
	}
	handler := &CloudPlanGatewayV1{Observed: &CloudObserveGatewayV1{Gateway: base, Observer: observer}}

	for _, token := range []string{"", "invalid_token"} {
		r := requestPlanV1(handler, "POST", "/cloud/v1/compute/plan", token, "application/json", validPlanBodyV1)
		if r.Code != 401 {
			t.Fatalf("invalid auth %d", r.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("unauthorized request queried node")
	}
	for _, tt := range []struct {
		method, path, ct, body string
		want                   int
	}{
		{"GET", "/cloud/v1/compute/plan", "application/json", validPlanBodyV1, 405},
		{"POST", "/cloud/v1/compute/plan?target=evil", "application/json", validPlanBodyV1, 400},
		{"POST", "/cloud/v1/compute/plan", "text/plain", validPlanBodyV1, 415},
		{"POST", "/cloud/v1/compute/plan", "application/json", `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0],"reward":1}`, 400},
		{"POST", "/cloud/v1/compute/plan", "application/json", `{"batch_size":1,"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`, 400},
		{"POST", "/cloud/v1/compute/plan", "application/json", strings.Repeat(" ", MaxSubmitBodyBytesV1+1), 413},
	} {
		r := requestPlanV1(handler, tt.method, tt.path, testCloudTokenA, tt.ct, tt.body)
		if r.Code != tt.want {
			t.Errorf("%s %s returned %d want %d: %s", tt.method, tt.path, r.Code, tt.want, r.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests reached node: %d", calls.Load())
	}

	var firstID, firstHash string
	for i := 0; i < 2; i++ {
		r := requestPlanV1(handler, "POST", "/cloud/v1/compute/plan", testCloudTokenA, "application/json", validPlanBodyV1)
		if r.Code != 200 {
			t.Fatalf("plan %d: %s", r.Code, r.Body.String())
		}
		var result struct {
			Mode            string `json:"mode"`
			TaskID          string `json:"task_id"`
			InputHash       string `json:"input_hash"`
			Workload        string `json:"workload"`
			ChainID         string `json:"chain_id"`
			NodeHeight      uint64 `json:"node_height"`
			DispatchEnabled bool   `json:"dispatch_enabled"`
			ComputeEnabled  bool   `json:"compute_enabled"`
			EscrowEnabled   bool   `json:"escrow_enabled"`
			ProofsAvailable bool   `json:"proofs_available"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Mode != "dry_run" || result.Workload != "ml_inference_quantized" || result.ChainID != testNodeChainID || result.NodeHeight != 42 || result.TaskID == "" || result.InputHash == "" || result.DispatchEnabled || result.ComputeEnabled || result.EscrowEnabled || result.ProofsAvailable {
			t.Fatalf("unsafe plan response: %+v", result)
		}
		if i == 0 {
			firstID, firstHash = result.TaskID, result.InputHash
		} else if firstID != result.TaskID || firstHash != result.InputHash {
			t.Fatal("non-deterministic plan")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("expected two status GETs, got %d", calls.Load())
	}
	if store.Count("tenant-a") != 0 || store.Count("tenant-b") != 0 {
		t.Fatal("plan unexpectedly persisted jobs")
	}
	existing := requestPlanV1(handler, "GET", "/cloud/v1/capabilities", testCloudTokenA, "", "")
	if existing.Code != 200 {
		t.Fatalf("existing observer route regressed: %d", existing.Code)
	}
	missing := requestPlanV1(handler, "GET", "/cloud/v1/jobs/job_00000000000000000000000000000000", testCloudTokenA, "", "")
	if missing.Code != 404 {
		t.Fatalf("existing jobs route regressed: %d", missing.Code)
	}
}

func TestCloudPlanGatewayObserverFailsClosed(t *testing.T) {
	base, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "private", "jobs.json"))
	defer store.Close()
	disabled := &CloudPlanGatewayV1{Observed: &CloudObserveGatewayV1{Gateway: base}}
	r := requestPlanV1(disabled, "POST", "/cloud/v1/compute/plan", testCloudTokenA, "application/json", validPlanBodyV1)
	if r.Code != 503 {
		t.Fatalf("disabled observer: %d", r.Code)
	}
	port, closeNode := makeNodeServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Replace(validNodeStatusJSON(), testNodeChainID, "prism-other", 1))
	})
	observer, err := NewCloudNodeObserverV1(port, testNodeChainID, testNodeGenesisHash)
	if err != nil {
		t.Fatal(err)
	}
	wrong := &CloudPlanGatewayV1{Observed: &CloudObserveGatewayV1{Gateway: base, Observer: observer}}
	r = requestPlanV1(wrong, "POST", "/cloud/v1/compute/plan", testCloudTokenA, "application/json", validPlanBodyV1)
	if r.Code != 502 || !bytes.Contains(r.Body.Bytes(), []byte("node_identity_mismatch")) {
		t.Fatalf("wrong network accepted: %d %s", r.Code, r.Body.String())
	}
	closeNode()
	r = requestPlanV1(wrong, "POST", "/cloud/v1/compute/plan", testCloudTokenA, "application/json", validPlanBodyV1)
	if r.Code != 502 {
		t.Fatalf("offline node accepted: %d", r.Code)
	}
	if store.Count("tenant-a") != 0 {
		t.Fatal("failed plans created jobs")
	}
}
