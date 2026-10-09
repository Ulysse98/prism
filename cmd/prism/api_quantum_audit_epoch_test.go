package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"prism/internal/usefulwork"
)

func TestQuantumAuditPolicyEpochHTTP(t *testing.T) {
	api, job, proof, verifiers, mux := quantumAuditHTTPFixture(t)

	path := "/api/v1/compute/jobs/" +
		job.ID + "/quantum-audit"

	policy := *api.quantumAuditPolicy
	epoch := api.quantumAuditEpoch

	first, err := usefulwork.SignQuantumPolicyBoundReport(
		proof, verifiers[0], policy, epoch,
	)
	if err != nil {
		t.Fatal(err)
	}

	second, err := usefulwork.SignQuantumPolicyBoundReport(
		proof, verifiers[1], policy, epoch,
	)
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := usefulwork.SignQuantumVerificationReport(
		proof, verifiers[0],
	)
	if err != nil {
		t.Fatal(err)
	}

	oldEpoch, err := usefulwork.SignQuantumPolicyBoundReport(
		proof, verifiers[0], policy, epoch-1,
	)
	if err != nil {
		t.Fatal(err)
	}

	changedPolicy := policy
	changedPolicy.RequiredApprovals = 3

	changed, err := usefulwork.SignQuantumPolicyBoundReport(
		proof, verifiers[0], changedPolicy, epoch,
	)
	if err != nil {
		t.Fatal(err)
	}

	tampered := first
	tampered.PolicyEpoch++

	tests := []struct {
		name     string
		reports  any
		wantCode int
	}{
		{
			name: "ValidTwoOfThree",
			reports: []usefulwork.QuantumPolicyBoundReport{
				first, second,
			},
			wantCode: http.StatusOK,
		},
		{
			name: "RejectLegacyV1Directly",
			reports: []usefulwork.QuantumVerificationReport{
				legacy,
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "RejectOldEpoch",
			reports: []usefulwork.QuantumPolicyBoundReport{
				oldEpoch,
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "RejectChangedThreshold",
			reports: []usefulwork.QuantumPolicyBoundReport{
				changed,
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "RejectTamperedEpoch",
			reports: []usefulwork.QuantumPolicyBoundReport{
				tampered,
			},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Rate limiting is tested separately.
			api.quantumAuditRate.mu.Lock()
			api.quantumAuditRate.arrivals = nil
			api.quantumAuditRate.mu.Unlock()

			before := snapshotQuantumAuditHTTP(t, api)

			body, err := json.Marshal(map[string]any{
				"reports": tc.reports,
			})
			if err != nil {
				t.Fatal(err)
			}

			request := httptest.NewRequest(
				http.MethodPost, path, bytes.NewReader(body),
			)
			request.Header.Set(
				"Authorization", "Bearer "+quantumAuditTestToken,
			)

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantCode {
				t.Fatalf(
					"got HTTP %d want %d: %s",
					recorder.Code,
					tc.wantCode,
					recorder.Body.String(),
				)
			}

			if tc.wantCode == http.StatusOK {
				var response struct {
					Audit         usefulwork.QuantumAuditResult `json:"audit"`
					Observational bool                          `json:"observational"`
				}

				if err := json.Unmarshal(
					recorder.Body.Bytes(), &response,
				); err != nil {
					t.Fatal(err)
				}

				if !response.Observational ||
					response.Audit.PolicyEpoch != epoch ||
					!response.Audit.QuorumSatisfied ||
					response.Audit.Status !=
						usefulwork.QuantumAuditProvisional {
					t.Fatalf(
						"wrong policy epoch audit result: %+v",
						response,
					)
				}
			}

			after := snapshotQuantumAuditHTTP(t, api)

			if !reflect.DeepEqual(before, after) {
				t.Fatal("policy-epoch audit changed Prism state")
			}
		})
	}

	// Fail closed if the trusted epoch is missing at runtime.
	api.quantumAuditEpoch = 0

	disabled := performQuantumAuditSecurityRequest(mux, path)
	if disabled.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"missing policy epoch: got HTTP %d",
			disabled.Code,
		)
	}
}
