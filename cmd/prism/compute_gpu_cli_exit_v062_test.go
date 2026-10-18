package main

import "testing"

func TestGPUBackendRequestedV062(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"gpu separate", []string{"-compute-backend", "gpu"}, true},
		{"gpu equals", []string{"-compute-backend=gpu"}, true},
		{"gpu double dash", []string{"--compute-backend=gpu"}, true},
		{"cpu default", []string{"Bob", "job"}, false},
		{"cpu explicit", []string{"-compute-backend", "cpu"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gpuBackendRequestedV062(tc.args); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGPUSessionSingleJobRequestedV063(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"single gpu session", []string{
			"-compute-backend", "gpu-session", "Bob", "job",
		}, true},
		{"equals syntax", []string{
			"--compute-backend=gpu-session", "Bob", "job",
		}, true},
		{"autonomous loop", []string{
			"-compute-backend=gpu-session", "-auto", "-poll=5s", "Bob",
		}, false},
		{"autonomous equals", []string{
			"-compute-backend=gpu-session", "-auto=true", "Bob",
		}, false},
		{"auto explicitly false", []string{
			"-compute-backend=gpu-session", "-auto=false", "Bob", "job",
		}, true},
		{"default cpu", []string{"Bob", "job"}, false},
		{"old gpu", []string{"-compute-backend=gpu"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gpuSessionSingleJobRequestedV063(tc.args)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrismVersionV063(t *testing.T) {
	if prismVersion != "0.63.0" {
		t.Fatalf("unexpected Prism version: %s", prismVersion)
	}
}
