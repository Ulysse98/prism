package usefulwork

import (
	"fmt"
	"math"
)

const BellReferenceAlpha = 0.001

type BellReferenceReport struct {
	Model        string
	Shots        uint64
	Alpha        float64
	ObservedP00  float64
	MaxDeviation float64
}

func VerifyBellReference(
	counts []uint64,
	shots uint64,
) (BellReferenceReport, error) {

	report := BellReferenceReport{
		Model: "bell-ideal-v1",
		Shots: shots,
		Alpha: BellReferenceAlpha,
	}

	if len(counts) != BellCountLength {
		return report, fmt.Errorf("expected exactly 4 Bell counts")
	}

	if shots == 0 || shots > 1_000_000 {
		return report, fmt.Errorf("invalid reference shot count: %d", shots)
	}

	var total uint64
	for _, count := range counts {
		if count > shots-total {
			return report, fmt.Errorf("Bell counts exceed shots")
		}
		total += count
	}

	if total != shots {
		return report, fmt.Errorf("Bell counts total mismatch")
	}

	// Ideal Bell simulation has no 01 or 10 outcomes.
	if counts[BellCount01] != 0 || counts[BellCount10] != 0 {
		return report, fmt.Errorf("unexpected Bell off-diagonal measurements")
	}

	observed := float64(counts[BellCount00]) / float64(shots)

	// Two-sided Hoeffding bound for Bernoulli(p=0.5).
	tolerance := math.Sqrt(
		math.Log(2.0/BellReferenceAlpha) / (2.0 * float64(shots)),
	)

	report.ObservedP00 = observed
	report.MaxDeviation = tolerance

	if math.Abs(observed-0.5) > tolerance {
		return report, fmt.Errorf(
			"Bell distribution outside reference bound: p00=%f tolerance=%f",
			observed,
			tolerance,
		)
	}

	return report, nil
}
