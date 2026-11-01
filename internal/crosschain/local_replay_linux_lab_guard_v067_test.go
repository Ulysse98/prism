package crosschain

import (
	"runtime"
	"testing"
)

// requireLinuxReplayLabV067 keeps private Unix-filesystem laboratory tests
// on their qualified host, without hiding independent cross-platform tests.
func requireLinuxReplayLabV067(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("v0.67 local replay requires a Linux-local POSIX filesystem; non-Linux fails closed")
	}
}
