package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrismAPIStartupFailureCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "invalid port",
			args: []string{"-port", "0"},
		},
		{
			name: "epoch without policy",
			args: []string{
				"-quantum-audit-epoch", "1",
			},
		},
		{
			name: "policy without token",
			args: []string{
				"-quantum-audit-policy", "missing.json",
			},
		},
		{
			name: "missing state",
			args: []string{
				"-data",
				filepath.Join(t.TempDir(), "missing"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := runAPICommand(tc.args)

			if code != 1 {
				t.Fatalf(
					"expected exit 1, got %d", code,
				)
			}
		})
	}
}

func TestPrismAPIExitStatusChild(t *testing.T) {
	if os.Getenv("PRISM_V059_EXIT_CHILD") != "1" {
		return
	}

	os.Args = []string{
		"prism", "api", "-port", "0",
	}

	main()

	t.Fatal("main returned without error exit")
}

func TestPrismAPIExitStatusProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		executable,
		"-test.run=^TestPrismAPIExitStatusChild$",
	)

	cmd.Env = append(
		os.Environ(),
		"PRISM_V059_EXIT_CHILD=1",
	)

	output, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		t.Fatal("CLI process timed out")
	}

	var exitErr *exec.ExitError

	if !errors.As(err, &exitErr) {
		t.Fatalf(
			"expected error exit, got %v: %s",
			err, output,
		)
	}

	if exitErr.ExitCode() != 1 {
		t.Fatalf(
			"expected exit 1, got %d: %s",
			exitErr.ExitCode(), output,
		)
	}

	if !strings.Contains(
		string(output), "Invalid API port:",
	) {
		t.Fatalf(
			"unexpected CLI output: %s", output,
		)
	}
}
