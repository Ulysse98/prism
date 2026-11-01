//go:build linux

package computecloud

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
)

var (
	validCloudTenantV1 = regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)
	validIdempotencyV1 = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	validCloudJobIDV1  = regexp.MustCompile(`^job_[0-9a-f]{32}$`)
)

type CloudAPIKeyV1 struct {
	TenantID string `json:"tenant_id"`
	SHA256   string `json:"sha256"`
}

type CloudGatewayV1 struct {
	Store *LocalCloudStoreV1
	Keys  []CloudAPIKeyV1
}

func (g *CloudGatewayV1) Validate() error {
	if g.Store == nil || len(g.Keys) == 0 || len(g.Keys) > 32 {
		return errors.New("invalid gateway configuration")
	}
	tenants := map[string]bool{}
	hashes := map[string]bool{}
	for _, key := range g.Keys {
		if !validCloudTenantV1.MatchString(key.TenantID) {
			return errors.New("invalid tenant identifier")
		}
		b, err := hex.DecodeString(key.SHA256)
		if err != nil || len(b) != 32 || hex.EncodeToString(b) != key.SHA256 {
			return errors.New("invalid key digest")
		}
		if tenants[key.TenantID] || hashes[key.SHA256] {
			return errors.New("duplicate key or tenant")
		}
		tenants[key.TenantID] = true
		hashes[key.SHA256] = true
	}
	return nil
}

func (g *CloudGatewayV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// No browser-facing CORS support; this is a local API for trusted clients.
	tenant, ok := g.authenticate(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Prism Cloud"`)
		errorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	path := r.URL.Path
	switch {
	case path == "/cloud/v1/jobs" && r.Method == http.MethodPost:
		g.createJob(w, r, tenant)
	case path == "/cloud/v1/usage" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"jobs_used": g.Store.Count(tenant), "jobs_limit": MaxJobsPerTenantV1})
	case strings.HasPrefix(path, "/cloud/v1/jobs/") && r.Method == http.MethodGet:
		tail := strings.TrimPrefix(path, "/cloud/v1/jobs/")
		isProof := strings.HasSuffix(tail, "/proof")
		if isProof {
			tail = strings.TrimSuffix(tail, "/proof")
		}
		if !validCloudJobIDV1.MatchString(tail) {
			errorJSON(w, http.StatusNotFound, "not_found")
			return
		}
		job, err := g.Store.Get(tenant, tail)
		if err != nil {
			errorJSON(w, http.StatusNotFound, "not_found")
			return
		}
		if isProof {
			errorJSON(w, http.StatusConflict, "proof_unavailable_in_simulation")
			return
		}
		writeJSON(w, http.StatusOK, job)
	case path == "/cloud/v1/jobs" || path == "/cloud/v1/usage" || strings.HasPrefix(path, "/cloud/v1/jobs/"):
		w.Header().Set("Allow", allowedMethodV1(path))
		errorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
	default:
		errorJSON(w, http.StatusNotFound, "not_found")
	}
}
func allowedMethodV1(path string) string {
	if path == "/cloud/v1/jobs" {
		return "POST"
	}
	return "GET"
}

func (g *CloudGatewayV1) authenticate(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", false
	}
	value := values[0]
	if !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(value, "Bearer ")
	if len(token) < 32 || len(token) > 128 || strings.ContainsAny(token, " \t\r\n") {
		return "", false
	}
	digest := sha256.Sum256([]byte(token))
	var tenant string
	for _, key := range g.Keys {
		expected, _ := hex.DecodeString(key.SHA256) // checked by Validate at startup
		if subtle.ConstantTimeCompare(digest[:], expected) == 1 {
			tenant = key.TenantID
		}
	}
	return tenant, tenant != ""
}

func (g *CloudGatewayV1) createJob(w http.ResponseWriter, r *http.Request, tenant string) {
	idems := r.Header.Values("Idempotency-Key")
	if len(idems) != 1 || !validIdempotencyV1.MatchString(idems[0]) {
		errorJSON(w, http.StatusBadRequest, "idempotency_key_required")
		return
	}
	ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || ct != "application/json" {
		errorJSON(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}
	if r.ContentLength > MaxSubmitBodyBytesV1 {
		errorJSON(w, http.StatusRequestEntityTooLarge, "body_too_large")
		return
	}
	bytes, err := io.ReadAll(io.LimitReader(r.Body, MaxSubmitBodyBytesV1+1))
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if len(bytes) > MaxSubmitBodyBytesV1 {
		errorJSON(w, http.StatusRequestEntityTooLarge, "body_too_large")
		return
	}
	task, err := ParseSubmitV1(bytes)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid_workload")
		return
	}
	job, created, err := g.Store.Create(tenant, idems[0], task.ID, task.InputHash)
	if errors.Is(err, ErrCloudConflict) {
		errorJSON(w, http.StatusConflict, "idempotency_conflict")
		return
	}
	if errors.Is(err, ErrCloudQuota) {
		errorJSON(w, http.StatusTooManyRequests, "quota_exceeded")
		return
	}
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "storage_error")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	w.Header().Set("Location", "/cloud/v1/jobs/"+job.ID)
	writeJSON(w, status, job)
}
func errorJSON(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
