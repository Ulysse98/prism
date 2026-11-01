//go:build linux

package computecloud

import (
	"errors"
	"net/http"
)

// CloudObserveGatewayV1 adds authenticated, read-only node observation.
// All existing job and usage routes delegate unchanged to CloudGatewayV1.
type CloudObserveGatewayV1 struct {
	Gateway  *CloudGatewayV1
	Observer *CloudNodeObserverV1 // nil means disabled
}

func (g *CloudObserveGatewayV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/cloud/v1/capabilities" && r.URL.Path != "/cloud/v1/node" {
		g.Gateway.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, ok := g.Gateway.authenticate(r); !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Prism Cloud"`)
		errorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		errorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if r.URL.RawQuery != "" {
		errorJSON(w, http.StatusBadRequest, "query_not_supported")
		return
	}
	if r.URL.Path == "/cloud/v1/capabilities" {
		writeJSON(w, http.StatusOK, struct {
			Mode             string `json:"mode"`
			Workload         string `json:"workload"`
			Features         int    `json:"features"`
			Classes          int    `json:"classes"`
			MaxBatch         uint64 `json:"max_batch"`
			NodeProbeEnabled bool   `json:"node_probe_enabled"`
			ComputeEnabled   bool   `json:"compute_enabled"`
			ProofsAvailable  bool   `json:"proofs_available"`
		}{
			Mode: "simulation", Workload: "ml_inference_quantized", Features: 3, Classes: 3,
			MaxBatch: MaxBatchSizeV1, NodeProbeEnabled: g.Observer != nil,
			ComputeEnabled: false, ProofsAvailable: false,
		})
		return
	}
	if g.Observer == nil {
		errorJSON(w, http.StatusServiceUnavailable, "node_probe_disabled")
		return
	}
	status, err := g.Observer.ReadStatus(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrCloudNodeMismatchV1):
			errorJSON(w, http.StatusBadGateway, "node_identity_mismatch")
		case errors.Is(err, ErrCloudNodeUnhealthyV1):
			errorJSON(w, http.StatusBadGateway, "node_unhealthy")
		default:
			errorJSON(w, http.StatusBadGateway, "node_unavailable")
		}
		return
	}
	writeJSON(w, http.StatusOK, status)
}
