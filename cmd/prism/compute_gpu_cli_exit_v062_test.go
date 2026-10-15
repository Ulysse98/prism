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
