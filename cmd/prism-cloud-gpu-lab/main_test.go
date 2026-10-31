//go:build linux

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCloudGPULabCLIOptInRequired(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{
		nil,
		{"-gpu-bin", "/tmp/x", "-sha256", strings.Repeat("0", 64)},
		{"-enable-local-gpu", "-gpu-bin", "/tmp/x", "-sha256", strings.Repeat("0", 64), "unexpected"},
	} {
		out.Reset()
		if err := run(context.Background(), args, strings.NewReader("{}"), &out); err == nil {
			t.Fatal("dangerous CLI args accepted")
		}
		if out.Len() != 0 {
			t.Fatal("unexpected output")
		}
	}
}
