package main

import (
	"strings"
	"testing"
)

func TestV061CanonicalBootstrapCommitRequiresExplicitConfirmation(t *testing.T) {
	if err := runNodeBootstrapTeamCommitV061([]string{}); err == nil {
		t.Fatal("a missing confirmation was accepted")
	}
	if err := runNodeBootstrapTeamCommitV061([]string{
		"-data", "not-a-real-data-dir", "-authorities", "Alice,Bob",
		"-signers", "Alice,Charlie", "-expected-tip", bootstrapV061Tip5,
		"-expected-bootstrap-id", strings.Repeat("0", 64),
		"-confirm-commit", "WRONG-CONFIRMATION",
	}); err == nil {
		t.Fatal("invalid confirmation accepted")
	}
}

func TestV061CanonicalBootstrapCommitRejectsMissingChain(t *testing.T) {
	err := runNodeBootstrapTeamCommitV061([]string{
		"-data", t.TempDir(), "-authorities", "Alice,Bob", "-signers", "Alice,Charlie",
		"-expected-tip", bootstrapV061Tip5,
		"-expected-bootstrap-id", strings.Repeat("0", 64),
		"-confirm-commit", bootstrapV061Confirm,
	})
	if err == nil {
		t.Fatal("missing chain was accepted")
	}
}
