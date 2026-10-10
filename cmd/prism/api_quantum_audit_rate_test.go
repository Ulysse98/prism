package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuantumAuditRateLimiterWindow(t *testing.T) {
	var limiter quantumAuditRateLimiter
	start := time.Unix(1700000000, 0)

	for i := 0; i < quantumAuditRateLimit; i++ {
		allowed, retry := limiter.allow(start)

		if !allowed || retry != 0 {
			t.Fatalf(
				"request %d unexpectedly rejected: %v",
				i+1, retry,
			)
		}
	}

	allowed, retry := limiter.allow(start)

	if allowed || retry != quantumAuditRateWindow {
		t.Fatalf(
			"expected full-window rejection, got %v, %v",
			allowed, retry,
		)
	}

	allowed, retry = limiter.allow(
		start.Add(quantumAuditRateWindow - time.Second),
	)

	if allowed || retry != time.Second {
		t.Fatalf(
			"expected one-second retry, got %v, %v",
			allowed, retry,
		)
	}

	allowed, retry = limiter.allow(
		start.Add(quantumAuditRateWindow),
	)

	if !allowed || retry != 0 {
		t.Fatal("expired audit window did not reopen")
	}
}

func TestQuantumAuditRateLimiterConcurrent(t *testing.T) {
	var limiter quantumAuditRateLimiter

	const workers = 32

	now := time.Unix(1700000000, 0)

	var accepted atomic.Int32
	var wait sync.WaitGroup

	for i := 0; i < workers; i++ {
		wait.Add(1)

		go func() {
			defer wait.Done()

			allowed, _ := limiter.allow(now)

			if allowed {
				accepted.Add(1)
			}
		}()
	}

	wait.Wait()

	if accepted.Load() != int32(quantumAuditRateLimit) {
		t.Fatalf(
			"expected %d approvals, got %d",
			quantumAuditRateLimit,
			accepted.Load(),
		)
	}
}

func TestQuantumAuditRateLimitHTTP(t *testing.T) {
	api, job, _, _, mux := quantumAuditHTTPFixture(t)

	path := "/api/v1/compute/jobs/" +
		job.ID + "/quantum-audit"

	before := snapshotQuantumAuditHTTP(t, api)

	for i := 0; i < quantumAuditRateLimit; i++ {
		recorder := performQuantumAuditSecurityRequest(
			mux, path,
		)

		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"request %d returned HTTP %d: %s",
				i+1,
				recorder.Code,
				recorder.Body.String(),
			)
		}
	}

	rejected := performQuantumAuditSecurityRequest(
		mux, path,
	)

	if rejected.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"expected HTTP 429, got %d: %s",
			rejected.Code,
			rejected.Body.String(),
		)
	}

	retry, err := strconv.Atoi(
		rejected.Header().Get("Retry-After"),
	)

	if err != nil || retry < 1 || retry > 60 {
		t.Fatalf("invalid Retry-After: %d, %v", retry, err)
	}

	// Authentication must still run before the exhausted quota.
	unauthorizedRequest := httptest.NewRequest(
		http.MethodPost,
		path,
		strings.NewReader(`{"reports":[]}`),
	)

	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, unauthorizedRequest)

	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected HTTP 401, got %d",
			unauthorized.Code,
		)
	}

	if api.quantumAuditBusy.Load() {
		t.Fatal("rate-limited request leaked audit slot")
	}

	after := snapshotQuantumAuditHTTP(t, api)

	if !reflect.DeepEqual(before, after) {
		t.Fatal("rate-limited audit changed Prism state")
	}
}
