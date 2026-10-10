package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"prism/internal/usefulwork"
)

func performQuantumAuditSecurityRequest(
	mux *http.ServeMux,
	path string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost,
		path,
		strings.NewReader(`{"reports":[]}`),
	)

	request.Header.Set(
		"Authorization",
		"Bearer "+quantumAuditTestToken,
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	return recorder
}

func TestQuantumAuditSingleFlight(t *testing.T) {
	api, job, _, _, mux := quantumAuditHTTPFixture(t)

	path := "/api/v1/compute/jobs/" +
		job.ID + "/quantum-audit"

	before := snapshotQuantumAuditHTTP(t, api)

	if !api.quantumAuditBusy.CompareAndSwap(false, true) {
		t.Fatal("audit should initially be available")
	}

	busy := performQuantumAuditSecurityRequest(mux, path)

	if busy.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"busy audit returned %d, want 429: %s",
			busy.Code,
			busy.Body.String(),
		)
	}

	if busy.Header().Get("Retry-After") != "1" {
		t.Fatal("missing Retry-After header")
	}

	if !api.quantumAuditBusy.Load() {
		t.Fatal("busy request released another audit's slot")
	}

	api.quantumAuditBusy.Store(false)

	available := performQuantumAuditSecurityRequest(mux, path)

	if available.Code != http.StatusOK {
		t.Fatalf(
			"available audit returned %d: %s",
			available.Code,
			available.Body.String(),
		)
	}

	if api.quantumAuditBusy.Load() {
		t.Fatal("successful request leaked audit slot")
	}

	after := snapshotQuantumAuditHTTP(t, api)

	if !reflect.DeepEqual(before, after) {
		t.Fatal("single-flight audit changed Prism state")
	}
}

func TestQuantumAuditReleasesStateLock(t *testing.T) {
	api, job, _, _, mux := quantumAuditHTTPFixture(t)

	task, err := usefulwork.NewQuantumSimulationTask(
		2, 4096,
	)
	if err != nil {
		t.Fatal(err)
	}

	pending, err := api.computeMarket.Create(
		task,
		"Alice",
		20,
		580203,
	)
	if err != nil {
		t.Fatal(err)
	}

	prefix := "/api/v1/compute/jobs/"
	suffix := "/quantum-audit"

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{
			name:     "MissingJob",
			path:     prefix + strings.Repeat("0", 64) + suffix,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "UnsettledJob",
			path:     prefix + pending.ID + suffix,
			wantCode: http.StatusConflict,
		},
		{
			name:     "CompletedAudit",
			path:     prefix + job.ID + suffix,
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotQuantumAuditHTTP(t, api)

			recorder := performQuantumAuditSecurityRequest(
				mux,
				tc.path,
			)

			if recorder.Code != tc.wantCode {
				t.Fatalf(
					"got HTTP %d, want %d: %s",
					recorder.Code,
					tc.wantCode,
					recorder.Body.String(),
				)
			}

			if !api.stateMu.TryLock() {
				t.Fatal("stateMu remained locked after audit")
			}
			api.stateMu.Unlock()

			if api.quantumAuditBusy.Load() {
				t.Fatal("audit slot remained occupied")
			}

			after := snapshotQuantumAuditHTTP(t, api)

			if !reflect.DeepEqual(before, after) {
				t.Fatal("audit changed Prism state")
			}
		})
	}
}
