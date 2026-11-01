//go:build linux

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestGPUGlobalLockCLIRequiresOptIn(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run(nil, &out, &errBuf); code == 0 || out.Len() != 0 ||
		!strings.Contains(errBuf.String(), "explicit opt-in") {
		t.Fatalf("unsafe default: code=%d stdout=%s stderr=%s", code, &out, &errBuf)
	}
}
func TestGPUGlobalLockCLIDurationIsBounded(t *testing.T) {
	for _, duration := range []string{"0s", "31s", "-1s"} {
		var out, errBuf bytes.Buffer
		start := time.Now()
		if code := run([]string{"-enable-global-gpu-lock", "-hold-for", duration}, &out, &errBuf); code == 0 {
			t.Fatalf("accepted invalid duration %s", duration)
		}
		if time.Since(start) > time.Second || out.Len() != 0 {
			t.Fatal("unsafe duration path")
		}
	}
}
