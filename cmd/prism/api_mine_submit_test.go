package main

import (
	"strings"
	"testing"
)

func TestValidateMineSourceHeightAcceptsCurrent(
	t *testing.T,
) {
	if err :=
		validateMineSourceHeight(
			44,
			44,
		); err != nil {

		t.Fatalf(
			"current PoUW height rejected: %v",
			err,
		)
	}
}

func TestValidateMineSourceHeightRejectsStale(
	t *testing.T,
) {
	err :=
		validateMineSourceHeight(
			45,
			44,
		)

	if err == nil {
		t.Fatal(
			"expected stale PoUW job rejection",
		)
	}

	if !strings.Contains(
		err.Error(),
		"stale PoUW job",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
