//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestNodeObserverEnvOptIn(t *testing.T) {
	for _, name := range []string{"PRISM_CLOUD_NODE_PORT", "PRISM_CLOUD_NODE_CHAIN_ID", "PRISM_CLOUD_NODE_GENESIS_HASH"} {
		t.Setenv(name, "")
	}
	observer, err := nodeObserverFromEnv()
	if err != nil || observer != nil {
		t.Fatalf("observer should default to disabled: %+v %v", observer, err)
	}
	t.Setenv("PRISM_CLOUD_NODE_PORT", "18064")
	if _, err := nodeObserverFromEnv(); err == nil {
		t.Fatal("accepted partial config")
	}
	t.Setenv("PRISM_CLOUD_NODE_CHAIN_ID", "prism-isolated-test")
	t.Setenv("PRISM_CLOUD_NODE_GENESIS_HASH", strings.Repeat("a", 64))
	observer, err = nodeObserverFromEnv()
	if err != nil || observer == nil {
		t.Fatalf("valid observer config failed: %v", err)
	}
	t.Setenv("PRISM_CLOUD_NODE_PORT", "99999")
	if _, err := nodeObserverFromEnv(); err == nil {
		t.Fatal("unsafe port accepted")
	}
	t.Setenv("PRISM_CLOUD_NODE_PORT", "18064")
	t.Setenv("PRISM_CLOUD_NODE_CHAIN_ID", "http://example.com")
	if _, err := nodeObserverFromEnv(); err == nil {
		t.Fatal("unsafe chain ID accepted")
	}
}
