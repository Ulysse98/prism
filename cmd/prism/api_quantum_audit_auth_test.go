package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestQuantumAuditAuthentication(t *testing.T) {
	api, job, _, _, mux := quantumAuditHTTPFixture(t)

	url := "/api/v1/compute/jobs/" +
		job.ID + "/quantum-audit"

	tests := []struct {
		name        string
		headers     []string
		disableHash bool
		wantCode    int
	}{
		{
			name: "ValidBearer",
			headers: []string{
				"Bearer " + quantumAuditTestToken,
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "MissingBearer",
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "InvalidBearer",
			headers: []string{
				"Bearer " + strings.Repeat("00", 32),
			},
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "MalformedBearer",
			headers:  []string{"Bearer bad"},
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "WrongScheme",
			headers: []string{
				"Basic " + quantumAuditTestToken,
			},
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "DuplicateAuthorization",
			headers: []string{
				"Bearer " + quantumAuditTestToken,
				"Bearer " + quantumAuditTestToken,
			},
			wantCode: http.StatusUnauthorized,
		},
		{
			name:        "NoConfiguredToken",
			disableHash: true,
			headers: []string{
				"Bearer " + quantumAuditTestToken,
			},
			wantCode: http.StatusServiceUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshotQuantumAuditHTTP(t, api)

			savedHash := api.quantumAuditTokenHash
			if tc.disableHash {
				api.quantumAuditTokenHash = nil
			}

			defer func() {
				api.quantumAuditTokenHash = savedHash
			}()

			request := httptest.NewRequest(
				http.MethodPost,
				url,
				strings.NewReader(`{"reports":[]}`),
			)

			for _, value := range tc.headers {
				request.Header.Add("Authorization", value)
			}

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantCode {
				t.Fatalf(
					"got HTTP %d, want %d: %s",
					recorder.Code,
					tc.wantCode,
					recorder.Body.String(),
				)
			}

			if tc.wantCode == http.StatusUnauthorized &&
				recorder.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing WWW-Authenticate header")
			}

			after := snapshotQuantumAuditHTTP(t, api)

			if !reflect.DeepEqual(before, after) {
				t.Fatal("authentication changed Prism state")
			}
		})
	}
}

func TestLoadQuantumAuditToken(t *testing.T) {
	validToken := quantumAuditTestToken

	cases := []struct {
		name  string
		data  string
		valid bool
	}{
		{"Valid", validToken, true},
		{"ValidWithNewline", validToken + "\n", true},
		{"Short", "abcd", false},
		{"NonHex", strings.Repeat("z", 64), false},
		{"Empty", "", false},
		{"Oversized", strings.Repeat("a", 129), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(
				t.TempDir(),
				"audit-token.secret",
			)

			if err := os.WriteFile(
				path,
				[]byte(tc.data),
				0600,
			); err != nil {
				t.Fatal(err)
			}

			hash, err := loadQuantumAuditToken(path)

			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}

				decoded, err := hex.DecodeString(validToken)
				if err != nil {
					t.Fatal(err)
				}

				expected := sha256.Sum256(decoded)

				if hash != expected {
					t.Fatal("unexpected token hash")
				}
			} else if err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}
