package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteAPIMetrics(t *testing.T) {
	snapshot := apiMetricsSnapshot{
		Height:      42,
		Blocks:      43,
		Validators:  3,
		TotalStake:  1000,
		TotalSupply: 2507,
		ChainValid:  true,
	}

	var output bytes.Buffer

	writeAPIMetrics(
		&output,
		snapshot,
	)

	expected := []string{
		"# TYPE prism_chain_height gauge",
		"prism_chain_height 42",
		"# TYPE prism_chain_blocks gauge",
		"prism_chain_blocks 43",
		"# TYPE prism_validators gauge",
		"prism_validators 3",
		"# TYPE prism_total_stake gauge",
		"prism_total_stake 1000",
		"# TYPE prism_total_supply gauge",
		"prism_total_supply 2507",
		"# TYPE prism_chain_valid gauge",
		"prism_chain_valid 1",
	}

	for _, value := range expected {
		if !strings.Contains(
			output.String(),
			value,
		) {
			t.Fatalf(
				"missing metric %q in:\n%s",
				value,
				output.String(),
			)
		}
	}
}

func TestWriteAPIMetricsReportsInvalidChain(t *testing.T) {
	var output bytes.Buffer

	writeAPIMetrics(
		&output,
		apiMetricsSnapshot{
			ChainValid: false,
		},
	)

	if !strings.Contains(
		output.String(),
		"prism_chain_valid 0",
	) {
		t.Fatalf(
			"expected invalid-chain metric in:\n%s",
			output.String(),
		)
	}
}
