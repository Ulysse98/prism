package main

import (
	"testing"

	"prism/internal/compute"
)

func registerComputeMarketplaceCleanup(
	t *testing.T,
	market *compute.Marketplace,
) {
	t.Helper()

	t.Cleanup(func() {
		if err := market.Close(); err != nil {
			t.Errorf(
				"cannot close compute marketplace: %v",
				err,
			)
		}
	})
}
