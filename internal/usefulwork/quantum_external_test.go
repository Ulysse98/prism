package usefulwork

import (
	"strings"
	"testing"
)

func validQuantumWorkerResponse() quantumWorkerResponse {
	return quantumWorkerResponse{
		OK:       true,
		Backend:  "cudaq:nvidia",
		Workload: "bell",
		Shots:    4096,
		Counts: map[string]uint64{
			"00": 2055,
			"01": 0,
			"10": 0,
			"11": 2041,
		},
	}
}

func TestValidateQuantumWorkerResponseValid(t *testing.T) {
	response := validQuantumWorkerResponse()
	counts, err := validateQuantumWorkerResponse(response, "nvidia", 4096)
	if err != nil {
		t.Fatalf("valid worker response rejected: %v", err)
	}
	if len(counts) != 4 || counts[0] != 2055 || counts[3] != 2041 {
		t.Fatalf("unexpected counts: %v", counts)
	}
}

func TestValidateQuantumWorkerResponseRejectsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*quantumWorkerResponse)
		want   string
	}{
		{
			name: "FakeBackend",
			mutate: func(r *quantumWorkerResponse) {
				r.Backend = "cudaq:fake"
			},
			want: "backend",
		},
		{
			name: "WrongWorkload",
			mutate: func(r *quantumWorkerResponse) {
				r.Workload = "fake"
			},
			want: "workload",
		},
		{
			name: "ExtraState",
			mutate: func(r *quantumWorkerResponse) {
				r.Counts["XX"] = 999
			},
			want: "exactly 4 states",
		},
		{
			name: "MissingState",
			mutate: func(r *quantumWorkerResponse) {
				delete(r.Counts, "11")
			},
			want: "states",
		},
		{
			name: "WrongShots",
			mutate: func(r *quantumWorkerResponse) {
				r.Shots = 8192
			},
			want: "shots",
		},
		{
			name: "WorkerFailure",
			mutate: func(r *quantumWorkerResponse) {
				r.OK = false
				r.Error = "execution failed"
			},
			want: "execution failed",
		},
		{
			name: "InvalidDistribution",
			mutate: func(r *quantumWorkerResponse) {
				r.Counts["00"] = 1024
				r.Counts["01"] = 1024
				r.Counts["10"] = 1024
				r.Counts["11"] = 1024
			},
			want: "Bell",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := validQuantumWorkerResponse()
			tc.mutate(&response)

			_, err := validateQuantumWorkerResponse(response, "nvidia", 4096)
			if err == nil {
				t.Fatal("expected invalid worker response to be rejected")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected error: %v; want %q", err, tc.want)
			}
		})
	}
}
