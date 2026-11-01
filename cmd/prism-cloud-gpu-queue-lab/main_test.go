//go:build linux

package main

import (
	"bytes"
	"context"
	"testing"
)

func TestCloudGPUQueueCLIRequiresExplicitOptIn(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), nil, bytes.NewBuffer(nil), &out)
	if err == nil || out.Len() != 0 {
		t.Fatalf("unauthorized offline CLI accepted: %v", err)
	}
}

func TestCloudGPUQueueCLIStrictBatchInputs(t *testing.T) {
	for _, tc := range []struct{ json, ids string }{
		{`[]`, "one,two"},
		{`{}`, "one,two"},
		{`[{},{}] trailing`, "one,two"},
		{`[{},{}]`, "one"},
		{`[{},{}]`, "one,two,three"},
		{`[{}, {}, {}, {}, {}]`, "one,two,three,four,five"},
	} {
		if _, err := decodeApprovedWork([]byte(tc.json), tc.ids); err == nil {
			t.Fatalf("accepted invalid batch: %s", tc.json)
		}
	}
}
