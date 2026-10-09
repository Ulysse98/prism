package usefulwork

import "testing"

func TestVerifyBellReferenceValid(t *testing.T) {
	cases := []struct {
		name   string
		counts []uint64
		shots  uint64
	}{
		{"CUDAQSample", []uint64{2055, 0, 0, 2041}, 4096},
		{"Balanced", []uint64{2048, 0, 0, 2048}, 4096},
		{"OneShot", []uint64{1, 0, 0, 0}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := VerifyBellReference(tc.counts, tc.shots)
			if err != nil {
				t.Fatalf("valid Bell result rejected: %v", err)
			}

			if report.Shots != tc.shots || report.Model != "bell-ideal-v1" {
				t.Fatalf("unexpected reference report: %+v", report)
			}
		})
	}
}

func TestVerifyBellReferenceRejectsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		counts []uint64
		shots  uint64
	}{
		{"OffDiagonal", []uint64{2047, 1, 0, 2048}, 4096},
		{"StatisticalSkew", []uint64{2293, 0, 0, 1803}, 4096},
		{"WrongTotal", []uint64{2048, 0, 0, 2047}, 4096},
		{"MissingCount", []uint64{2048, 0, 2048}, 4096},
		{"ExcessCount", []uint64{4097, 0, 0, 0}, 4096},
		{"ZeroShots", []uint64{0, 0, 0, 0}, 0},
		{"ExcessiveShots", []uint64{500001, 0, 0, 500000}, 1000001},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyBellReference(tc.counts, tc.shots)
			if err == nil {
				t.Fatal("expected invalid result to be rejected")
			}
		})
	}
}

func TestVerifyBellReferenceStricterThanLegacy(t *testing.T) {
	counts := []uint64{2293, 0, 0, 1803}

	// Legacy fixed 8% threshold accepts this distribution.
	if err := VerifyBellCounts(counts, 4096); err != nil {
		t.Fatalf("unexpected legacy rejection: %v", err)
	}

	// Reference statistical verifier rejects it.
	if _, err := VerifyBellReference(counts, 4096); err == nil {
		t.Fatal("reference verifier accepted statistical skew")
	}
}
