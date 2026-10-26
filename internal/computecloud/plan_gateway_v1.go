//go:build linux

package computecloud

import (
	"errors"
	"io"
	"mime"
	"net/http"
)

// CloudPlanGatewayV1 adds a strictly non-dispatching compute preflight route.
// It delegates all pre-existing Cloud and observer routes without modification.
// A preflight is NOT a job, quote, proof, reservation, or authorization to spend.
type CloudPlanGatewayV1 struct {
	Observed *CloudObserveGatewayV1
}

func (g *CloudPlanGatewayV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const path = "/cloud/v1/compute/plan"
	if g == nil || g.Observed == nil || g.Observed.Gateway == nil || g.Observed.Gateway.Store == nil {
		errorJSON(w, http.StatusServiceUnavailable, "gateway_unavailable")
		return
	}
	if r.URL.Path != path {
		g.Observed.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Authorization MUST precede inspecting client content and querying the node.
	if _, ok := g.Observed.Gateway.authenticate(r); !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Prism Cloud"`)
		errorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		errorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if r.URL.RawQuery != "" {
		errorJSON(w, http.StatusBadRequest, "query_not_supported")
		return
	}
	if g.Observed.Observer == nil {
		errorJSON(w, http.StatusServiceUnavailable, "node_probe_disabled")
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		errorJSON(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}
	if r.Body == nil {
		errorJSON(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if r.ContentLength > MaxSubmitBodyBytesV1 {
		errorJSON(w, http.StatusRequestEntityTooLarge, "body_too_large")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxSubmitBodyBytesV1+1))
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if len(data) > MaxSubmitBodyBytesV1 {
		errorJSON(w, http.StatusRequestEntityTooLarge, "body_too_large")
		return
	}
	task, err := ParseSubmitV1(data)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid_workload")
		return
	}
	status, err := g.Observed.Observer.ReadStatus(r.Context())
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

	// No marketplace call, persistent job, wallet, reward, escrow, GPU, or proof.
	writeJSON(w, http.StatusOK, struct {
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
	}{
		Mode: "dry_run", TaskID: task.ID, InputHash: task.InputHash,
		Workload: task.Type, ChainID: status.ChainID, NodeHeight: status.Height,
		DispatchEnabled: false, ComputeEnabled: false,
		EscrowEnabled: false, ProofsAvailable: false,
	})
}
