//go:build linux

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLocalBrokerCLIRequiresOptInAndFullPinning(t *testing.T) {
	testCases := [][]string{
		nil,
		{"-enable-local-broker-gpu"},
		{"-enable-local-broker-gpu", "-gpu-bin", "/tmp/gpu"},
		{"-enable-local-broker-gpu", "-gpu-bin", "/tmp/gpu", "-sha256", strings.Repeat("a", 64), "-node-port", "18065"},
		{"-enable-local-broker-gpu", "-gpu-bin", "/tmp/gpu", "-sha256", strings.Repeat("a", 64), "-node-port", "18065", "-chain-id", "prism-test", "-genesis-hash", strings.Repeat("b", 64), "-confirm-task-id", "foo", "extra"},
	}
	for _, args := range testCases {
		var output bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader("{}"), &output); err == nil {
			t.Fatalf("unsafe CLI args were accepted: %q", args)
		}
		if output.Len() != 0 {
			t.Fatal("unsafe args generated output")
		}
	}
}
