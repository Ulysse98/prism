//go:build linux

package computecloud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const validCloudPayload = `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`
const otherCloudPayload = `{"batch_size":1,"inputs":[-1,0,1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`
const testCloudTokenA = "pc_dev_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testCloudTokenB = "pc_dev_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func makeCloudGateway(t *testing.T, path string) (*CloudGatewayV1, *LocalCloudStoreV1) {
	t.Helper()
	store, err := OpenLocalCloudStoreV1(path)
	if err != nil {
		t.Fatal(err)
	}
	key := func(tenant, token string) CloudAPIKeyV1 {
		hash := sha256.Sum256([]byte(token))
		return CloudAPIKeyV1{TenantID: tenant, SHA256: hex.EncodeToString(hash[:])}
	}
	g := &CloudGatewayV1{Store: store, Keys: []CloudAPIKeyV1{key("tenant-a", testCloudTokenA), key("tenant-b", testCloudTokenB)}}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	return g, store
}
func cloudReq(g *CloudGatewayV1, method, path, token, idempotency, payload string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(payload))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotency != "" {
		req.Header.Set("Idempotency-Key", idempotency)
	}
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(rr, req)
	return rr
}
func decodeCloudJob(t *testing.T, rr *httptest.ResponseRecorder) CloudJobV1 {
	t.Helper()
	var j CloudJobV1
	if err := json.Unmarshal(rr.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	return j
}
func TestCloudGatewayAuthAndIsolation(t *testing.T) {
	g, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "store", "jobs.json"))
	defer store.Close()
	for _, token := range []string{"", "wrong", testCloudTokenA + "junk"} {
		r := cloudReq(g, "POST", "/cloud/v1/jobs", token, "a1", validCloudPayload)
		if r.Code != 401 {
			t.Fatalf("invalid key gave %d", r.Code)
		}
	}
	created := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "a1", validCloudPayload)
	if created.Code != 201 {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	job := decodeCloudJob(t, created)
	if job.ID == "" || job.TaskID == "" || job.InputHash == "" || job.Status != "SIMULATED" {
		t.Fatalf("invalid job %+v", job)
	}
	if strings.Contains(created.Body.String(), "tenant-a") {
		t.Fatal("tenant leakage")
	}
	if rr := cloudReq(g, "GET", "/cloud/v1/jobs/"+job.ID, testCloudTokenB, "", ""); rr.Code != 404 {
		t.Fatalf("cross-tenant read=%d", rr.Code)
	}
	if rr := cloudReq(g, "GET", "/cloud/v1/jobs/"+job.ID+"/proof", testCloudTokenB, "", ""); rr.Code != 404 {
		t.Fatalf("cross-tenant proof=%d", rr.Code)
	}
	if rr := cloudReq(g, "GET", "/cloud/v1/jobs/"+job.ID+"/proof", testCloudTokenA, "", ""); rr.Code != 409 {
		t.Fatalf("simulated proof=%d", rr.Code)
	}
	if rr := cloudReq(g, "GET", "/cloud/v1/usage", testCloudTokenB, "", ""); !strings.Contains(rr.Body.String(), `"jobs_used":0`) {
		t.Fatal("tenant usage leak")
	}
}
func TestCloudIdempotencyAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store", "jobs.json")
	g, store := makeCloudGateway(t, path)
	first := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "retry-1", validCloudPayload)
	if first.Code != 201 {
		t.Fatalf("first: %d %s", first.Code, first.Body.String())
	}
	original := decodeCloudJob(t, first)
	second := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "retry-1", validCloudPayload)
	if second.Code != 200 || decodeCloudJob(t, second).ID != original.ID {
		t.Fatal("idempotent repeat changed result")
	}
	if rr := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "retry-1", otherCloudPayload); rr.Code != 409 {
		t.Fatalf("conflict %d", rr.Code)
	}
	if rr := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenB, "retry-1", otherCloudPayload); rr.Code != 201 {
		t.Fatalf("different tenant idempotence: %d", rr.Code)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	g2, store2 := makeCloudGateway(t, path)
	defer store2.Close()
	restart := cloudReq(g2, "POST", "/cloud/v1/jobs", testCloudTokenA, "retry-1", validCloudPayload)
	if restart.Code != 200 || decodeCloudJob(t, restart).ID != original.ID {
		t.Fatal("restart lost idempotency")
	}
}
func TestCloudBadInputAndQuota(t *testing.T) {
	g, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "store", "jobs.json"))
	defer store.Close()
	for _, test := range []struct {
		key, body string
		status    int
	}{
		{"", validCloudPayload, 400},
		{"key1", `{"batch_size":1,"requester":"Alice","inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`, 400},
		{"key2", strings.Repeat("A", MaxSubmitBodyBytesV1+1), 413},
		{"key3", `{"batch_size":1,"inputs":[1,2,3]}`, 400},
		{"bad key", validCloudPayload, 400},
	} {
		rr := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, test.key, test.body)
		if rr.Code != test.status {
			t.Fatalf("case %q got %d: %s", test.key, rr.Code, rr.Body.String())
		}
	}
	for i := 0; i < MaxJobsPerTenantV1; i++ {
		r := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, fmt.Sprintf("key-%d", i), validCloudPayload)
		if r.Code != 201 {
			t.Fatalf("job %d: %d %s", i, r.Code, r.Body.String())
		}
	}
	if rr := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "over-limit", validCloudPayload); rr.Code != 429 {
		t.Fatalf("quota %d", rr.Code)
	}
	if rr := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "key-0", validCloudPayload); rr.Code != 200 {
		t.Fatalf("retry over quota %d", rr.Code)
	}
	if rr := cloudReq(g, "GET", "/cloud/v1/jobs/../../etc/passwd", testCloudTokenA, "", ""); rr.Code != 404 {
		t.Fatalf("path traversal %d", rr.Code)
	}
	if rr := cloudReq(g, "GET", "/cloud/v1/usage", testCloudTokenA, "", ""); !strings.Contains(rr.Body.String(), `"jobs_used":32`) {
		t.Fatalf("usage %s", rr.Body.String())
	}
}
func TestCloudConcurrentIdempotence(t *testing.T) {
	g, store := makeCloudGateway(t, filepath.Join(t.TempDir(), "store", "jobs.json"))
	defer store.Close()
	const n = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := map[string]int{}
	failures := []string{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := cloudReq(g, "POST", "/cloud/v1/jobs", testCloudTokenA, "concurrent", validCloudPayload)
			mu.Lock()
			defer mu.Unlock()
			if rr.Code != 200 && rr.Code != 201 {
				failures = append(failures, rr.Body.String())
				return
			}
			var job CloudJobV1
			if json.Unmarshal(rr.Body.Bytes(), &job) != nil {
				failures = append(failures, "invalid JSON")
				return
			}
			ids[job.ID]++
		}()
	}
	wg.Wait()
	if len(failures) > 0 || len(ids) != 1 || store.Count("tenant-a") != 1 {
		t.Fatalf("race failures=%v ids=%v", failures, ids)
	}
}
func TestCloudStoreExclusiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store", "jobs.json")
	_, first := makeCloudGateway(t, path)
	defer first.Close()
	if other, err := OpenLocalCloudStoreV1(path); err == nil {
		other.Close()
		t.Fatal("second writer allowed")
	}
}
func TestCloudHeaderSmuggling(t *testing.T) {
	g, s := makeCloudGateway(t, filepath.Join(t.TempDir(), "store", "jobs.json"))
	defer s.Close()
	req := httptest.NewRequest("POST", "/cloud/v1/jobs", bytes.NewBufferString(validCloudPayload))
	req.Header.Add("Authorization", "Bearer "+testCloudTokenA)
	req.Header.Add("Authorization", "Bearer "+testCloudTokenB)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "abc")
	rr := httptest.NewRecorder()
	g.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("multiple Authorization accepted: %d", rr.Code)
	}
	req2 := httptest.NewRequest("POST", "/cloud/v1/jobs", io.NopCloser(strings.NewReader(validCloudPayload)))
	req2.Header.Set("Authorization", "Bearer "+testCloudTokenA)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Add("Idempotency-Key", "abc")
	req2.Header.Add("Idempotency-Key", "def")
	rr2 := httptest.NewRecorder()
	g.ServeHTTP(rr2, req2)
	if rr2.Code != 400 {
		t.Fatalf("multiple idempotency accepted: %d", rr2.Code)
	}
}
